# 状态机

禁止业务代码散落 UPDATE status。状态推进必须通过 transition 函数。

## Project

opportunity -> bidding -> compliance_review -> submitted -> closed。

closed 后 result 为 won / lost / pending。won 后允许创建 cost_project。

## Bid Document

draft -> generating -> editing -> in_review -> approved -> submitted -> archived。

审批驳回：in_review -> editing。

## Bid Chapter

pending -> generating -> generated -> accepted -> edited -> needs_fix。

## Generation Job

queued -> running -> paused -> running -> done / failed / cancelled。

## Compliance Issue

pass / warn / fail_candidate / fail。LLM 只能产生 warn 或 fail_candidate；fail 必须由规则或人工确认产生。

## Export

queued -> running -> done / failed / cancelled。

## Knowledge Document

uploaded -> parsing -> chunking -> embedding -> indexed -> failed。
