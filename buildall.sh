for f in $(find . -name build.sh)
do (cd $(dirname $f) || exit; echo $(dirname $f); ./build.sh)
done