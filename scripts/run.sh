F=$1
shift
docker run -e "SRC=`cat $F`" -e "ARGS=$*" dboulytchev/pl-platforms
