package chain

import (
	"embed"
	"io/fs"
)

//go:embed templates/*.yaml
var templatesFS embed.FS

// Template 按名返回内置 chain 模板内容；不存在返回 ok=false。
func Template(name string) (string, bool) {
	raw, err := fs.ReadFile(templatesFS, "templates/"+name+".yaml")
	if err != nil {
		return "", false
	}
	return string(raw), true
}
