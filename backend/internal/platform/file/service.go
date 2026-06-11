package file

import (
	"fmt"

	"github.com/google/uuid"
)

func ObjectKey(tenantID, bizType string) string {
	return fmt.Sprintf("%s/%s/%s", tenantID, bizType, uuid.NewString())
}
