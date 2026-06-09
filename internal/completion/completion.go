// Package completion 生成各 shell 的补全脚本，并提供一键安装/卸载到 shell 配置文件。
package completion

import "fmt"

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
