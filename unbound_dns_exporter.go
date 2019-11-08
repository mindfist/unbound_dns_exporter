package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	unboundHistogram = prometheus.NewDesc(
		prometheus.BuildFQName("unbound", "", "response_time_seconds"),
		"Query response time in seconds.", nil, nil)

	unboundMetrics = []*unboundMetric{}
)

type unboundMetric struct {
	desc      *prometheus.Desc
	valueType prometheus.ValueType
	pattern   *regexp.Regexp
}

type UboundExporter struct {
	socketFamily string
	host         string
	tlsConfig    tls.Config
}

func newUnboundMetric(name string, description string, valueType prometheus.ValueType, labels []string, pattern string) *unboundMetric {
	return &unboundMetric{
		desc: prometheus.NewDesc(prometheus.BuildFQName("unbound", "", name),
			description,
			labels, nil),
		valueType: valueType,
		pattern:   regexp.MustCompile(pattern),
	}
}

func CollectFromReader(file io.Reader, ch chan<- prometheus.Metric) error {
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanBytes)
	histogramPattern := regexp.MustCompile("^histogram\\.\\d+\\.d+\\.to\\.(\\d+\\.\\d+)$")

	histogramCount := uint64(0)
	histogramAvg := float64(0)
	histogramBuckets := make(map[float64]uint64)

	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "=")
		if len(fields) != 2 {
			return fmt.Errorf(
				"%q is not a valid key-value pair", scanner.Text())

		}

		for _, metric := range unboundMetrics {
			if matches := metric.pattern.FindStringSubmatch(fields[0]); matches != nil {
				value, err := strconv.ParseFloat(fields[1], 64)

				if err != nil {
					return err
				}

				ch <- prometheus.MustNewConstMetric(
					metric.desc,
					metric.valueType,
					value,
					matches[1:]...)
				break
			}
		}

		if matches := histogramPattern.FindStringSubmatch(fields[0]); matches != nil {
			end, err := strconv.ParseFloat(matches[1], 64)
			if err != nil {
				return err
			}

			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return err
			}

			histogramBuckets[end] = value
			histogramCount += value

		} else if fields[0] == "total.recursion.time.avg" {
			value, err := strconv.ParseFloat(fields[1], 64)
			if err != nil {
				return err
			}

			histogramAvg = value
		}
	}

	// Convert the metrics to a cumulative Prometheus histogram.
	// Reconstruct the sum of all samples from the average value
	// provided by Unbound.

	keys := []float64{}
	for k := range histogramBuckets {
		keys = append(keys, k)
	}
	sort.Float64s(keys)
	prev := uint64(0)
	for _, i := range keys {
		histogramBuckets[i] += prev
		prev = histogramBuckets[i]
	}

	ch <- prometheus.MustNewConstHistogram(
		unboundHistogram,
		histogramCount,
		histogramAvg*float64(histogramCount),
		histogramBuckets)

	return scanner.Err()
}

func main() {

}
