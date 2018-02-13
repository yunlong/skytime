x=0
while [ $x -lt 100 ]
do
    boom -n 10 -c 1 -m POST -d "{\"peers\":\"$1\", \"br_msg\": \"asdfasdfasdsdfgsdfhgdfgsddfgsdfg\"}" http://127.0.0.1:8000/api/broadcast_peers
    #boom -n 10 -c 1 -m POST -d "{\"peers\":\"$1\", \"br_msg\": \"asdfasdfasdsdfgsdfhgdfgsddfgsdfg\"}" http://10.187.102.92:8000/api/broadcast_peers
    #boom -n 100 -c 10 -m POST -d "I HATE GNU" http://127.0.0.1:8000/api/broadcast_all
    #boom -n 100 -c 10 -m POST -d "I HATE GNU" http://10.187.102.92:8000/api/broadcast_all
    let x=x+1
done
