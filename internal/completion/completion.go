// Package completion 生成各 shell 的补全脚本，并提供一键安装/卸载到 shell 配置文件。
package completion

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// userHomeDir 便于测试替换。
var userHomeDir = os.UserHomeDir

const (
	markerStart = "# >>> ccr completion >>>"
	markerEnd   = "# <<< ccr completion <<<"
)

// Script 返回指定 shell 的补全脚本。支持 bash / zsh / powershell。
// 脚本运行时调用 `ccr __complete_names` 取动态的 profile 名/别名。
func Script(shell string) (string, error) {
	switch shell {
	case "bash":
		return bashScript, nil
	case "zsh":
		return zshScript, nil
	case "powershell", "pwsh":
		return pwshScript, nil
	default:
		return "", fmt.Errorf("不支持的 shell：%q（支持 bash/zsh/powershell）", shell)
	}
}

const bashScript = `# ccr bash 补全
_ccr() {
    local cur cmds names
    cur="${COMP_WORDS[COMP_CWORD]}"
    if [ "$COMP_CWORD" -eq 1 ]; then
        cmds="ls show edit alias unalias default completion -h --help -v --version"
        names="$(ccr __complete_names 2>/dev/null)"
        COMPREPLY=( $(compgen -W "${cmds} ${names}" -- "${cur}") )
    fi
}
complete -F _ccr ccr
`

const zshScript = `#compdef ccr
_ccr() {
    local -a cmds names
    cmds=(ls show edit alias unalias default completion)
    names=(${(f)"$(ccr __complete_names 2>/dev/null)"})
    _describe 'command' cmds
    _describe 'profile' names
}
compdef _ccr ccr
`

const pwshScript = `# ccr PowerShell 补全
Register-ArgumentCompleter -Native -CommandName ccr -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)
    $cmds = @('ls','show','edit','alias','unalias','default','completion')
    $names = @(ccr __complete_names 2>$null)
    @($cmds + $names) | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
    }
}
`

// rcPath 返回某 shell 的配置文件路径（相对 home）。
func rcPath(shell string) (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", err
	}
	switch shell {
	case "bash":
		return filepath.Join(home, ".bashrc"), nil
	case "zsh":
		return filepath.Join(home, ".zshrc"), nil
	case "powershell", "pwsh":
		// 跨平台简化：统一放 Documents/PowerShell/profile.ps1。
		return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"), nil
	default:
		return "", fmt.Errorf("不支持的 shell：%q", shell)
	}
}

// loadLine 返回引导块中间那行（source / Invoke-Expression）。
// 先做存在性兜底：ccr 不在 PATH 上时静默跳过，避免卸载/换版本后污染 shell 启动。
func loadLine(shell string) string {
	switch shell {
	case "powershell", "pwsh":
		return "if (Get-Command ccr -ErrorAction SilentlyContinue) { ccr completion powershell | Out-String | Invoke-Expression }"
	default:
		return fmt.Sprintf("command -v ccr >/dev/null 2>&1 && source <(ccr completion %s)", shell)
	}
}

// block 返回带标记的完整引导块。
func block(shell string) string {
	return fmt.Sprintf("%s\n%s\n%s\n", markerStart, loadLine(shell), markerEnd)
}

// Install 把引导块幂等地追加到 shell rc 文件。返回是否改动、rc 路径。
func Install(shell string) (changed bool, path string, err error) {
	path, err = rcPath(shell)
	if err != nil {
		return false, "", err
	}
	existing, _ := os.ReadFile(path) // 缺文件视为空
	if strings.Contains(string(existing), markerStart) {
		return false, path, nil // 已装，幂等
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, path, err
	}
	out := string(existing)
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	out += block(shell)
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return false, path, err
	}
	return true, path, nil
}

// Uninstall 删掉 rc 文件中的引导块。返回是否改动、rc 路径。
func Uninstall(shell string) (changed bool, path string, err error) {
	path, err = rcPath(shell)
	if err != nil {
		return false, "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, path, nil // 没文件没块
	}
	s := string(raw)
	start := strings.Index(s, markerStart)
	end := strings.Index(s, markerEnd)
	if start < 0 || end < 0 || end < start {
		return false, path, nil
	}
	end += len(markerEnd)
	if end < len(s) && s[end] == '\n' {
		end++
	}
	out := s[:start] + s[end:]
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return false, path, err
	}
	return true, path, nil
}

// DetectShell 从环境猜当前 shell：$SHELL 的 basename，Windows 回退 powershell。
func DetectShell() string {
	if sh := os.Getenv("SHELL"); sh != "" {
		base := filepath.Base(sh)
		switch {
		case strings.Contains(base, "zsh"):
			return "zsh"
		case strings.Contains(base, "bash"):
			return "bash"
		}
	}
	if os.Getenv("PSModulePath") != "" {
		return "powershell"
	}
	return ""
}
