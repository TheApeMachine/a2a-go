package metrics

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewStreamingMetrics(t *testing.T) {
	Convey("When creating a new metrics instance", t, func() {
		m := NewStreamingMetrics()
		Convey("Then it should not be nil", func() {
			So(m, ShouldNotBeNil)
		})
	})
}

func TestRecordConnection(t *testing.T) {
	Convey("Given a metrics instance", t, func() {
		m := NewStreamingMetrics()
		m.RecordConnection(true, time.Second)
		Convey("Then connection stats are recorded", func() {
			So(m.TotalConnections, ShouldEqual, 1)
			So(m.FailedConnections, ShouldEqual, 0)
		})
	})
}

func TestRecordReconnection(t *testing.T) {
	Convey("Given a metrics instance", t, func() {
		m := NewStreamingMetrics()
		m.RecordReconnection()
		Convey("Then reconnections increase", func() {
			So(m.Reconnections, ShouldEqual, 1)
		})
	})
}

func TestRecordEvent(t *testing.T) {
	Convey("Given a metrics instance", t, func() {
		m := NewStreamingMetrics()
		m.RecordEvent(false, time.Second, time.Second)
		Convey("Then event metrics update", func() {
			So(m.TotalEvents, ShouldEqual, 1)
			So(m.DroppedEvents, ShouldEqual, 0)
		})
	})
}

func TestGetMetrics(t *testing.T) {
	Convey("Given a metrics instance with data", t, func() {
		m := NewStreamingMetrics()
		m.RecordConnection(true, time.Second)
		m.RecordEvent(false, time.Second, time.Second)
		metrics := m.GetMetrics()
		Convey("Then returned metrics reflect counts", func() {
			So(metrics["total_connections"], ShouldEqual, int64(1))
			So(metrics["total_events"], ShouldEqual, int64(1))
		})
	})
}

func TestReset(t *testing.T) {
	Convey("Given a populated metrics instance", t, func() {
		m := NewStreamingMetrics()
		m.RecordConnection(true, time.Second)
		m.RecordEvent(false, time.Second, time.Second)
		m.RecordReconnection()
		m.Reset()
		Convey("Then all values are cleared", func() {
			So(m.TotalConnections, ShouldEqual, 0)
			So(m.Reconnections, ShouldEqual, 0)
			So(m.TotalEvents, ShouldEqual, 0)
		})
	})
}
