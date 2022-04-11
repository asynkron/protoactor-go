go get ./...

# shellcheck disable=SC2044
for f in $(find . -name go.mod)
do (cd $(dirname $f) || exit; go mod tidy)
done
