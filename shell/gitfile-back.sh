#!/bin/sh

. md5sum.txt > /dev/null
: > md5sum.txt

needBakup=0

for i in `ls ..`
do
    filename=`basename $i`
    extension="${filename#*.}"
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
            echo "# "$i >> md5sum.txt

            fileNameMd5=`echo $i | md5sum | cut -d ' ' -f 1`
            fileNameVar="_"$fileNameMd5
            fileOldMd5=${!fileNameVar}
            fileNewMd5=`md5sum ../$i | cut -d ' ' -f 1`
            if [ "$fileOldMd5" != "$fileNewMd5" ];then
                if [ "$fileOldMd5" ];then
                    echo -e "\033[32m $i modified,backup it \033[0m"
                else
                    echo -e "\033[31m $i created,backup it \033[0m"
                fi
                cp ../$i ./
                echo $fileNameVar=$fileNewMd5 >> md5sum.txt

                needBakup=1
            else
                if [ ! -e $i ];then
                    cp ../$i ./
                    neeBack=1
                fi
                echo $fileNameVar=$fileOldMd5 >> md5sum.txt
            fi

            echo "" >> md5sum.txt
            ;;
    esac
done

if [ 1 == $needBakup ];then
    echo -e "\033[33m backup files to gitfile.tar.gz \033[0m"
    cd ..
    cp gitfile.tar.gz gitfile-bak.tar.gz
    tar zcvf gitfile.tar.gz ./gitfile > /dev/null
else
    echo -e "\033[36m don't need to backup files \033[0m"
fi
