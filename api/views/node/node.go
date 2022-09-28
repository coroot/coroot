package node

import (
	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/model"
)

func Render(w *model.World, node *model.Node) *model.AuditReport {
	return auditor.AuditNode(w, node)
}
