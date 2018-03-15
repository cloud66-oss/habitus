package build

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

func readDockerfileToTarget(dockerfile string, target string) (string, error) {
	targetPattern := regexp.MustCompile(fmt.Sprintf(`\s+[aA][sS]\s+%s\s*$`, target))
	nextPattern := regexp.MustCompile(`\s+[aA][sS]\s+.+\s*$`)

	scanner := bufio.NewScanner(strings.NewReader(dockerfile))
	buf := bytes.NewBuffer(make([]byte, 0, len(dockerfile)))

	targetFound := false
	for scanner.Scan() {
		line := scanner.Text()
		// Read dockerfile until the end of the target stage
		if targetFound && nextPattern.MatchString(line) {
			break
		} else if targetPattern.MatchString(line) {
			targetFound = true
		}
		buf.WriteString(line)
		buf.WriteString("\n")
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("[bug] failed to process generated dockerfile: %v", err)
	}
	if !targetFound {
		return "", fmt.Errorf(`build target "%s" does not exist in Dockerfile - double-check that the target is not misspelled`, target)
	}
	return buf.String(), nil
}
