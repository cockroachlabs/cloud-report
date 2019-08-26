SERVER=$1
if [ -z "$SERVER" ]
then
      echo "error: please specify internal IP of other machine"
      exit
fi

ping $SERVER > network-ping.log &
sleep 60
PINGPID=$(pidof ping)
kill -2 $PINGPID
