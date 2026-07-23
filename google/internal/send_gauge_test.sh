				LabelsRefs: []uint32{
					1, 2,   // __name__ = my_gauge_metric
					3, 4,   // version = v1.2.3
					5, 6,   // job = my_job
					7, 8,   // instance = my_instance
					9, 10,  // project_id = gpe-test-1
					11, 12, // location = us-central1
					13, 14, // cluster = test-cluster
					15, 16, // namespace = test-namespace
					17, 18, // handler = test
				},
				Samples: []*writev2.Sample{
					{
						Value:     42.0,
						Timestamp: now,
					},
				},
				Metadata: &writev2.Metadata{
					Type:    writev2.Metadata_METRIC_TYPE_GAUGE,
					HelpRef: 19,
				},
			},
		},
	}
	data, err := proto.Marshal(req)
	if err != nil {
		log.Fatalf("proto.Marshal failed: %v", err)
	}
	compressed := snappy.Encode(nil, data)
	outFile := os.Args[1]
	if err := os.WriteFile(outFile, compressed, 0644); err != nil {
		log.Fatalf("os.WriteFile failed: %v", err)
	}
	log.Printf("Encoded and compressed Remote Write 2.0 Request (METRIC_TYPE_GAUGE) to %s (timestamp=%d)", outFile, now)
}
EOF
echo "Encoding Remote Write 2.0 Request (METRIC_TYPE_GAUGE) using Go inside $PROMETHEUS_ROOT..."
(cd "$PROMETHEUS_ROOT" && go run "$TMP_GO" "$TMP_SNAPPY")
# 2. Send the write request to staging-monitoring.sandbox.googleapis.com
echo "Sending Remote Write 2.0 request to staging endpoint for project gpe-test-1..."
curl -X POST \
  "https://staging-monitoring.sandbox.googleapis.com/v1/prometheus/api/v1/write" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/x-protobuf;proto=io.prometheus.write.v2.Request" \
  -H "Content-Encoding: snappy" \
  -H "X-Prometheus-Remote-Write-Version: 2.0.0" \
  -H "X-Goog-User-Project: gpe-test-1" \
  --data-binary "@$TMP_SNAPPY" -v
echo ""
echo "=========================================================="
echo "Querying verification TimeSeries from gpe-test-1 staging:"
echo "=========================================================="
# 3. Query the TimeSeries to verify ingestion
curl -G \
  "https://staging-monitoring.sandbox.googleapis.com/v3/projects/gpe-test-1/timeSeries" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "X-Goog-User-Project: gpe-test-1" \
  --data-urlencode "filter=metric.type=\"prometheus.googleapis.com/my_gauge_metric/gauge\"" \
  --data-urlencode "interval.startTime=$(date -u -d '15 minutes ago' +%Y-%m-%dT%H:%M:%SZ)" \
  --data-urlencode "interval.endTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
bwplotka@bwplotkai.c.googlers.com
gcert: 15h 37m

Agent State Debug

Incognito
2026.07.14.04_RC02