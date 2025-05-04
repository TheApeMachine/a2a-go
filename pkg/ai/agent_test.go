package ai

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/a2a-go/pkg/a2a"
)

func TestAgent(t *testing.T) {
	Convey("Given an agent", t, func() {
		card := &a2a.AgentCard{
			Name: "Test Agent",
		}
		agent, err := NewAgentFromCard(card)
		So(err, ShouldBeNil)
		So(agent, ShouldNotBeNil)
	})
}
