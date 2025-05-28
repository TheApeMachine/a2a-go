package auth

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewRateLimiter(t *testing.T) {
	Convey("When creating a rate limiter", t, func() {
		rl := NewRateLimiter(2, time.Second)
		Convey("Then it initializes correctly", func() {
			So(rl, ShouldNotBeNil)
		})
	})
}

func TestRateLimiterAllow(t *testing.T) {
	Convey("Given a limiter with capacity 2", t, func() {
		rl := NewRateLimiter(2, time.Second)
		ok1 := rl.Allow()
		ok2 := rl.Allow()
		ok3 := rl.Allow()
		Convey("Then the third call should be limited", func() {
			So(ok1, ShouldBeTrue)
			So(ok2, ShouldBeTrue)
			So(ok3, ShouldBeFalse)
		})
		time.Sleep(time.Second)
		Convey("And after waiting it allows again", func() {
			So(rl.Allow(), ShouldBeTrue)
		})
	})
}
