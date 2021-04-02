package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestSqlFile(t *testing.T) {
	f, err := os.OpenFile("test1.sql", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().Unix() * 1e3
	for i := 0; i < 1e3; i++ {
		var b bytes.Buffer
		b.WriteString("insert into test1 (age,name,ctime) values \n")
		for j := 0; j < 1e4; j++ {
			age := i*1e3 + j
			name := "name" + strconv.Itoa(age)
			b.WriteString(fmt.Sprintf("(%d,'%s',%d)", age, name, now))
			if j < 1e3-1 {
				b.WriteString(",\n")
			}
		}
		b.WriteString(";\n")
		f.WriteString(b.String())
		f.Sync()
		now += 24 * 60 * 60 * 1e3
	}
	f.Close()
}
