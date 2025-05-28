package registry

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRegisterTool(t *testing.T) {
	Convey("Given an empty registry", t, func() {
		registryMu.Lock()
		toolRegistry = make(map[string]ToolDefinition)
		registryMu.Unlock()

		def := ToolDefinition{SkillID: "dev", ToolName: "terminal"}
		RegisterTool(def)

		Convey("Then the tool can be retrieved", func() {
			got, ok := GetToolDefinition("dev")
			So(ok, ShouldBeTrue)
			So(got.ToolName, ShouldEqual, "terminal")
		})
	})
}

func TestGetToolDefinition(t *testing.T) {
	Convey("Given a registry with a tool", t, func() {
		registryMu.Lock()
		toolRegistry = map[string]ToolDefinition{"dev": {SkillID: "dev", ToolName: "terminal"}}
		registryMu.Unlock()

		Convey("When retrieving the tool", func() {
			got, ok := GetToolDefinition("dev")
			Convey("Then it should exist", func() {
				So(ok, ShouldBeTrue)
				So(got.SkillID, ShouldEqual, "dev")
			})
		})
	})
}
