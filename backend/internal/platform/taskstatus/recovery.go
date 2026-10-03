package taskstatus

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// ExpireInterruptedTasks is the final safety net for the SQL commit -> AI submit
// crash window, or permanently undeliverable callbacks. It uses the migration
// connection so all tenants are swept; every resource update retains tenant scope.
func ExpireInterruptedTasks(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `
        select id::text, tenant_id::text, resource_type, coalesce(resource_id::text, '')
        from ai_tasks where status in ('queued','running')
          and created_at < now() - interval '1 hour'
        order by created_at limit 100 for update skip locked`)
	if err != nil {
		return err
	}
	type expired struct{ id, tenant, resource, target string }
	tasks := []expired{}
	for rows.Next() {
		var task expired
		if err := rows.Scan(&task.id, &task.tenant, &task.resource, &task.target); err != nil {
			rows.Close()
			return err
		}
		tasks = append(tasks, task)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if _, err = tx.ExecContext(ctx, `update ai_tasks set status='failed',
            error_message='任务超过处理时限或在重启中中断，请重试',
            result=jsonb_build_object('error_code','task_deadline_exceeded','retryable',true),
            completed_at=now(),updated_at=now() where tenant_id=$1::uuid and id=$2::uuid`, task.tenant, task.id); err != nil {
			return err
		}
		// A newer request for the same resource must not be overwritten.
		guard := ` and not exists(select 1 from ai_tasks newer, ai_tasks old
            where old.id=$3::uuid and newer.tenant_id=old.tenant_id
            and newer.resource_type=old.resource_type and newer.resource_id=old.resource_id
            and newer.created_at>old.created_at)`
		var query string
		switch task.resource {
		case "bid_chapter":
			query = `update bid_chapters set status='needs_fix',needs_human_input='["任务超时，请重新生成"]'::jsonb,updated_at=now()
                where tenant_id=$1::uuid and id=$2::uuid and status='generating'`
		case "bid_parse_result":
			query = `update bid_parse_results set status='failed',error_message='任务超时，请重新解读',updated_at=now()
                where tenant_id=$1::uuid and id=$2::uuid and status in ('queued','processing')`
		case "bid_export":
			query = `update bid_exports set status='failed',error_message='任务超时，请重新导出',completed_at=now(),updated_at=now()
                where tenant_id=$1::uuid and id=$2::uuid and status in ('queued','running')`
		case "knowledge_document":
			query = `update knowledge_documents set parse_status='failed',updated_at=now()
                where tenant_id=$1::uuid and id=$2::uuid and parse_status in ('queued','processing')`
		}
		if query != "" && task.target != "" {
			if _, err = tx.ExecContext(ctx, query+guard, task.tenant, task.target, task.id); err != nil {
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `update bid_generation_steps set status='failed',
            error_message='任务超时，请重试',completed_at=now(),updated_at=now()
            where tenant_id=$1::uuid and ai_task_id=$2::uuid and status in ('queued','running')`, task.tenant, task.id); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `update bid_generation_jobs job set status='failed',
        error_message='生成步骤超时，请重试',completed_at=now(),updated_at=now()
        where job.status in ('queued','running') and exists(
            select 1 from bid_generation_steps step where step.tenant_id=job.tenant_id and step.job_id=job.id and step.status='failed')`); err != nil {
		return err
	}
	return tx.Commit()
}

func RunRecovery(ctx context.Context, db *sql.DB) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		sweepCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		if err := ExpireInterruptedTasks(sweepCtx, db); err != nil && ctx.Err() == nil {
			log.Printf("AI task timeout recovery failed: %v", err)
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
