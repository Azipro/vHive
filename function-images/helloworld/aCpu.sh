#! /bin/sh
endless_loop()
{
    echo -ne "i=0;
    while true
    do
    i=i+100;
    i=100
    done" | /bin/sh &
}


for i in 0 1 2 3 4
do
    endless_loop
done

/usr/local/bin/python /aMem.py &

/usr/local/bin/python /server.py