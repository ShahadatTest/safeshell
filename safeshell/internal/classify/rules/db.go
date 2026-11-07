package rules

import (
	"regexp"
	"strings"

	"safeshell/internal/shellparse"
)

func ruleDB(c shellparse.Command, f flags) []Finding {
	text := f.joined() + " " + c.Stdin
	up := strings.ToUpper(text)
	switch {
	case c.Name == "redis-cli" && (strings.Contains(up, "FLUSHALL") || strings.Contains(up, "FLUSHDB")):
		return one(fnd(R3, c, "Erases every key in the database."))
	case matches(sqlDropRe, text):
		return one(fnd(R3, c, "Drops or empties tables or databases. The data is gone unless there is a backup."))
	case matches(sqlDeleteRe, text) && !strings.Contains(up, " WHERE "):
		return one(fnd(R3, c, "Changes or deletes EVERY row, because there is no WHERE condition."))
	case matches(sqlDeleteRe, text):
		return one(fnd(R2, c, "Changes or deletes rows in a database."))
	case strings.Contains(up, "INSERT") || strings.Contains(up, "ALTER") || strings.Contains(up, "CREATE"):
		return one(fnd(R2, c, "Changes database contents or structure."))
	case strings.Contains(up, "SELECT") || strings.Contains(up, "\\DT") || strings.Contains(up, "SHOW"):
		return one(fnd(R0, c, "Only reads data."))
	}
	return one(fnd(R2, c, "Opens a database session. What you type inside it is not checked."))
}

func matches(pattern, s string) bool { return regexp.MustCompile(pattern).MatchString(s) }

const (
	sqlDropRe   = `(?i)\bdrop\s+(database|schema|table)\b|\btruncate\b`
	sqlDeleteRe = `(?i)\b(delete\s+from|update\s+\w+\s+set)\b`
)
