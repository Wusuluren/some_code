#!/bin/sh

#a simple tree

rootDir=.

foreach()
{
	cd $1
	local tabNumber=$(($2+5))
	for i in `ls`
	do
		if [ -d $i ];then
			printf "%${tabNumber}c $i:dir\n" ' '
			foreach $i $tabNumber
		elif [ -f $i ]; then
			printf "%${tabNumber}c $i:file\n" ' '
		fi
	done
	cd ..
}

main()
{
	foreach $rootDir 0
}


if [ $1 ];then
	rootDir=$1
fi
main
