package main

import (
	"fmt"
	"math/rand"
	"time"
)

//go:noinline
func getRandomNumber(i int64) (num int64) {
	num = int64(rand.Int())%100
	return num
}

func main() {
	for i:=int64(0);;i++{
		num:=getRandomNumber(i)
		if num<50{
			fmt.Println("pass", i, num)
		}else{
			fmt.Println("reject", i, num)
		}
		time.Sleep(time.Second)
	}
}
