#!/bin/bash

set -e

WORKDIR=$(mktemp -d)
echo "Working directory: $WORKDIR"

cleanup() {
    rm -rf "$WORKDIR"
}
trap cleanup EXIT

run_benchmark() {
    local dir=$1
    local name=$2
    local output_file=$3
    
    echo "Running benchmark for $name..."
    (cd "$dir" && go test -bench=. -benchmem -count=3 ./... 2>&1) | tee "$output_file"
}

parse_benchmark_results() {
    local file=$1
    local name=$2
    
    awk -v name="$name" '
    /^Benchmark/ {
        test=$1
        gsub(/Benchmark/, "", test)
        ns_op=$3
        gsub(/[^0-9.]/, "", ns_op)
        bytes_op=$5
        gsub(/[^0-9.]/, "", bytes_op)
        allocs_op=$7
        gsub(/[^0-9.]/, "", allocs_op)
        
        tests[test] = 1
        ns_sum[test] += ns_op
        bytes_sum[test] += bytes_op
        allocs_sum[test] += allocs_op
        count[test]++
    }
    END {
        for (test in tests) {
            printf "%s %s %.2f %.2f %.2f\n", test, name, \
                ns_sum[test]/count[test], \
                bytes_sum[test]/count[test], \
                allocs_sum[test]/count[test]
        }
    }
    ' "$file"
}

echo "=== Cloning repositories ==="

git clone --depth 1 https://github.com/noneback/go-taskflow.git "$WORKDIR/go-taskflow" 2>/dev/null
git clone --depth 1 https://github.com/zkep/flow.git "$WORKDIR/flow" 2>/dev/null

echo "=== Running benchmarks ==="

run_benchmark "$WORKDIR/go-taskflow/benchmark" "go-taskflow" "$WORKDIR/taskflow_bench.txt"
run_benchmark "$WORKDIR/flow/benchmark" "flow" "$WORKDIR/flow_bench.txt"

echo "=== Parsing results ==="

parse_benchmark_results "$WORKDIR/taskflow_bench.txt" "go-taskflow" > "$WORKDIR/taskflow_parsed.txt"
parse_benchmark_results "$WORKDIR/flow_bench.txt" "flow" > "$WORKDIR/flow_parsed.txt"

echo "=== Generating comparison table ==="

cat << 'EOF'
# benchmark comparison results

| case | go-taskflow (ns/op) | flow (ns/op) | go-taskflow (B/op) | flow (B/op) | go-taskflow (allocs/op) | flow (allocs/op) |
|----------|---------------------|--------------|--------------------|-------------|-------------------------|------------------|
EOF

join -1 1 -2 1 \
    <(sort -k1 "$WORKDIR/taskflow_parsed.txt") \
    <(sort -k1 "$WORKDIR/flow_parsed.txt") | \
awk '{
    test=$1
    tf_ns=$3
    tf_bytes=$4
    tf_allocs=$5
    f_ns=$7
    f_bytes=$8
    f_allocs=$9
    
    printf "| %s | %.0f | %.0f | %.0f | %.0f | %.1f | %.1f |\n", test, tf_ns, f_ns, tf_bytes, f_bytes, tf_allocs, f_allocs
}' | sort

echo ""
echo "Benchmark completed successfully!"
