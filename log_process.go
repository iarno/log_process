package main

import (
   "fmt"
   "strings"
   "time"
)

type logprocess struct {

   //channels进行通信
   rc chan string
   wc chan string

   //要读取日志的路径和influxdb的用户名、密码
   path string
   influxdb string

}


//1.日志读取
//使用l指针变量（也就是这个地址所指向的值）
func (l *logprocess) ReadLog()  {

    line := "Afe"

    l.rc <- line

}


//2.解析
func (l *logprocess)  Process() {

    data := <-l.rc

    //先进行个大写转换
    l.wc <- strings.ToUpper(data)

}


//3.写入influxdb中
func (l *logprocess)  WriteInfluxdb() {

   //输出
   fmt.Printf( <- l.wc)

}

func main() {

    lp := &logprocess{

       //使用make来
       rc: make(chan string),
       wc: make(chan string),

       path: "./access.log",
       influxdb: "username=liuli&password=123",

    }

    go lp.ReadLog()
    go lp.Process()
    go lp.WriteInfluxdb()

    //创建goroutine完后程序就自动退出  并不会等待
    time.Sleep(time.Second * 1)



}
