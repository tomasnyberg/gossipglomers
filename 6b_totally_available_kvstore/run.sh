go install . && ../maelstrom/maelstrom test \
    -w txn-rw-register \
    --bin ~/go/bin/maelstrom-txn \
    --node-count 1 \
    --time-limit 20 \
    --rate 1000 \
    --concurrency 2n \
    --consistency-models read-uncommitted \
    --availability total