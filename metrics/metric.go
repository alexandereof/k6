package metrics

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/guregu/null.v3"
)

// A Metric defines the shape of a set of data.
type Metric struct {
	Name     string     `json:"name"`
	Type     MetricType `json:"type"`
	Contains ValueType  `json:"contains"`

	// TODO: decouple the metrics from the sinks and thresholds... have them
	// linked, but not in the same struct?
	Tainted    null.Bool    `json:"tainted"`
	Thresholds Thresholds   `json:"thresholds"`
	Submetrics []*Submetric `json:"submetrics"`
	Sub        *Submetric   `json:"-"`
	Sink       Sink         `json:"-"`
	Observed   bool         `json:"-"`
}

// Sample samples the metric at the given time, with the provided tags and value
func (m *Metric) Sample(t time.Time, tags *SampleTags, value float64) Sample {
	return Sample{
		Time:   t,
		Tags:   tags,
		Value:  value,
		Metric: m,
	}
}

// New constructs a new Metric
func New(name string, typ MetricType, t ...ValueType) *Metric {
	vt := Default
	if len(t) > 0 {
		vt = t[0]
	}
	var sink Sink
	switch typ {
	case Counter:
		sink = &CounterSink{}
	case Gauge:
		sink = &GaugeSink{}
	case Trend:
		sink = &TrendSink{}
	case Rate:
		sink = &RateSink{}
	default:
		return nil
	}
	return &Metric{Name: name, Type: typ, Contains: vt, Sink: sink}
}

// A Submetric represents a filtered dataset based on a parent metric.
type Submetric struct {
	Name   string      `json:"name"`
	Suffix string      `json:"suffix"` // TODO: rename?
	Tags   *SampleTags `json:"tags"`

	Metric *Metric `json:"-"`
	Parent *Metric `json:"-"`
}

// AddSubmetric creates a new submetric from the key:value threshold definition
// and adds it to the metric's submetrics list.
func (m *Metric) AddSubmetric(keyValues string) (*Submetric, error) {
	keyValues = strings.TrimSpace(keyValues)
	if len(keyValues) == 0 {
		return nil, fmt.Errorf("submetric criteria for metric '%s' cannot be empty", m.Name)
	}
	kvs := strings.Split(keyValues, ",")
	rawTags := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		if kv == "" {
			continue
		}
		parts := strings.SplitN(kv, ":", 2)

		key := strings.Trim(strings.TrimSpace(parts[0]), `"'`)
		if len(parts) != 2 {
			rawTags[key] = ""
			continue
		}

		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		rawTags[key] = value
	}

	tags := IntoSampleTags(&rawTags)

	for _, sm := range m.Submetrics {
		if sm.Tags.IsEqual(tags) {
			return nil, fmt.Errorf(
				"sub-metric with params '%s' already exists for metric %s: %s",
				keyValues, m.Name, sm.Name,
			)
		}
	}

	subMetric := &Submetric{
		Name:   m.Name + "{" + keyValues + "}",
		Suffix: keyValues,
		Tags:   tags,
		Parent: m,
	}
	subMetricMetric := New(subMetric.Name, m.Type, m.Contains)
	subMetricMetric.Sub = subMetric // sigh
	subMetric.Metric = subMetricMetric

	m.Submetrics = append(m.Submetrics, subMetric)

	return subMetric, nil
}
