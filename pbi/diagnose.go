package pbi

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Diagnostic describes what the user must fix before syncing.
type Diagnostic struct {
	Code    string // azNotFound | azNotLoggedIn | tokenError
	Title   string
	Detail  string
	CmdHint string // command to copy-paste
}

// Diagnose checks whether the Azure CLI is installed and logged in.
func Diagnose(ctx context.Context) *Diagnostic {
	path, err := exec.LookPath("az")
	if err != nil {
		return &Diagnostic{
			Code:  "azNotFound",
			Title: "Azure CLI가 설치되어 있지 않습니다",
			Detail: "Power BI 데이터를 불러오려면 Azure CLI가 필요합니다. " +
				"아래 안내에 따라 설치한 뒤 로그인해 주세요.",
			CmdHint: "winget install --id Microsoft.AzureCLI -e",
		}
	}
	_ = path

	// Logged in? az account show fails when no profile exists.
	c, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	out, err := exec.CommandContext(c, "az", "account", "show", "--output", "tsv").CombinedOutput()
	if err != nil {
		return &Diagnostic{
			Code:  "azNotLoggedIn",
			Title: "Azure CLI 로그인이 필요합니다",
			Detail: fmt.Sprintf("Azure CLI는 설치되어 있지만 로그인되어 있지 않습니다. (%s)\n"+
				"아래 명령을 실행해 로그인해 주세요.", strings.TrimSpace(firstLine(string(out)))),
			CmdHint: "az login",
		}
	}
	return nil // all good
}

// firstLine returns the first non-empty line of s.
func firstLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			return t
		}
	}
	return ""
}
