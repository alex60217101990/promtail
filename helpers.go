package promtail

import (
	"fmt"
	"regexp"
	"runtime/debug"
	"sync"
)

var (
	packageOnceLoad                          sync.Once
	pattern, patternVersion, patternHostOrIP *regexp.Regexp
)

func init() {
	packageOnceLoad.Do(func() {
		pattern = regexp.MustCompile(`^[A-z-_0-9]+$`)
		patternHostOrIP = regexp.MustCompile(`^(https?://[A-z-_]+:?[0-9].)|([A-z-_]+:?[0-9].)|((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}:?[0-9]{2,})$`)
		patternVersion = regexp.MustCompile(`^[0-9]{1,2}\.[0-9]{1,2}\.[0-9]{1,3}$`)
	})
	fmt.Println("< INIT REGEXP >")
}

func sourceAndJobValidator(word *string) bool {
	return pattern.MatchString(*word)
}

func versionValidator(ver *string) bool {
	return patternVersion.MatchString(*ver)
}

func hostOrIPValidator(addr *string) bool {
	return patternHostOrIP.MatchString(*addr)
}

func LabelBuilder(label *Label) (string, error) {
	if label == nil {
		return "", fmt.Errorf("promtail_:_helpers_:_labelBuilder_: error: empty 'label' parameter, stack: %s", string(debug.Stack()))
	}
	if !hostOrIPValidator(&label.Host) {
		return "", fmt.Errorf("promtail_:_helpers_:_labelBuilder_: error: invalid label 'host', stack: %s", string(debug.Stack()))
	}
	if !sourceAndJobValidator(&label.Source) {
		return "", fmt.Errorf("promtail_:_helpers_:_labelBuilder_: error: invalid label 'source', stack: %s", string(debug.Stack()))
	}
	if !sourceAndJobValidator(&label.Job) {
		return "", fmt.Errorf("promtail_:_helpers_:_labelBuilder_: error: invalid label 'job', stack: %s", string(debug.Stack()))
	}
	return "{host=\"" + label.Host + "\",source=\"" + label.Source + "\",job=\"" + label.Job + "\"}", nil
}
