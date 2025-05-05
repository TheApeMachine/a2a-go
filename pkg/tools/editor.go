package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/viper"
)

type Editor struct {
	tool *mcp.Tool
}

func NewEditor() *mcp.Tool {
	v := viper.GetViper()
	baseKey := "tools.builder.params"

	tool := mcp.NewTool(
		v.GetString("tools.builder.name"),
		mcp.WithDescription(v.GetString("tools.builder.description")),
		mcp.WithString("filename",
			mcp.Description(v.GetString(baseKey+".filename.description")),
			func() mcp.PropertyOption {
				if v.GetBool(baseKey + ".filename.required") {
					return mcp.Required()
				}
				return nil
			}(),
		),
		mcp.WithString("find",
			mcp.Description(v.GetString(baseKey+".find.description")),
			func() mcp.PropertyOption {
				if v.GetBool(baseKey + ".find.required") {
					return mcp.Required()
				}
				return nil
			}(),
		),
		mcp.WithString("replace",
			mcp.Description(v.GetString(baseKey+".replace.description")),
			func() mcp.PropertyOption {
				if v.GetBool(baseKey + ".replace.required") {
					return mcp.Required()
				}
				return nil
			}(),
		),
		mcp.WithString("from",
			mcp.Description(v.GetString(baseKey+".from.description")),
			func() mcp.PropertyOption {
				if v.GetBool(baseKey + ".from.required") {
					return mcp.Required()
				}
				return nil
			}(),
		),
	)

	return &tool
}
