#!/bin/sh

cd ..
for i in `ls`
do
    filename=`basename $i`
    extension="${filename##*.}"
    case $extension in
        cpp);&
        c);&
        h);&
        rc);&
        inf);&
        sh);&
        py);&
        txt);&
        makefile);&
        SOURCES)
            echo "backup $i"
            cp $i gitfile/
            ;;
    esac
done

echo "backup tar.gz"
tar zcvf gitfile.tar.gz gitfile > /dev/null
