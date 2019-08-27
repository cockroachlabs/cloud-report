SERVER=$1
if [ -z "$SERVER" ]
then
      echo "error: please specify internal IP of other machine"
      exit
fi

# Run 5 pings per second
ping -i 0.2 $SERVER > network-ping.log &
sleep 60
PINGPID=$(pidof ping)

if ! [ -z "$PINGPID" ]
then
      kill -2 $PINGPID
fi
