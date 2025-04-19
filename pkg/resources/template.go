package resources

import (
    "fmt"
    "net/url"
    "regexp"
    "strings"
)

// ExpandTemplate replaces placeholders with concrete values.
func ExpandTemplate(tmpl string, vars map[string]string) (string, error) {
    res := tmpl
    for k, v := range vars {
        res = strings.ReplaceAll(res, "{"+k+"}", v)
    }
    if strings.Contains(res, "{") {
        return "", fmt.Errorf("not all variables provided for template %s", tmpl)
    }
    if _, err := url.Parse(res); err != nil {
        return "", fmt.Errorf("invalid uri after expansion: %w", err)
    }
    return res, nil
}

// matchTemplate tries to match a concrete uri against a template and returns
// a map of extracted variable values.
func matchTemplate(tmpl, uri string) (map[string]string, error) {
    vars := ParseTemplateVariables(tmpl)
    if len(vars) == 0 {
        return nil, fmt.Errorf("template contains no variables")
    }

    pattern := tmpl
    for _, v := range vars {
        pattern = strings.Replace(pattern, "{"+v.Name+"}", "([^/]+)", 1)
    }
    pattern = "^" + pattern + "$"

    re, err := regexp.Compile(pattern)
    if err != nil {
        return nil, err
    }
    m := re.FindStringSubmatch(uri)
    if m == nil {
        return nil, fmt.Errorf("uri does not match template")
    }
    out := map[string]string{}
    for i, v := range vars {
        out[v.Name] = m[i+1]
    }
    return out, nil
}
