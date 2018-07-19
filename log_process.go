package main

import (
   "fmt"
   "strings"
   "time"
	"os"
	"log"
    "bufio"
    "io"
)

//为了可扩展性  定义两个读写接口

//读取的接口
type Reader interface {
    //定义个方法
    Read(rc chan []byte)
}

//写入的接口
type Writer interface {
    //定义个方法
    Write(wc chan string)
}

type logprocess struct {

   //channels进行通信
   rc chan []byte
   wc chan string

   //要读取日志的路径和influxdb的用户名、密码
   read  Reader
   write Writer

}


//1.日志读取
//使用l指针变量（也就是这个地址所指向的值）
//文件路径结构体
type ReadFilePath struct {
    path string
}

//写入配置结构体
type WriteDb struct {
	db string
}

func (r *ReadFilePath) Read(rc chan []byte)  {

   f,err := os.Open(r.path)

   if err != nil {
       log.Println("Open file failed:", err)
   }
   //从文件末尾开始从行读取
   f.Seek(0,2)
   rd := bufio.NewReader(f)

   for {
       //读取文件每一行
       line,err := rd.ReadBytes('\n')
       if err == io.EOF {
           time.Sleep(500 * time.Millisecond)
           continue
       } else if err != nil {
           panic(fmt.Sprintf("ReadBytes error:%s", err.Error()))
       }

       rc <- line

   }





}

//2.解析
func (l *logprocess)  Process() {

    for v := range l.rc {
        //先进行个大写转换
        l.wc <- strings.ToUpper(string(v))
    }


}

//3.写入influxdb中
func (w *WriteDb)  Write(wc chan string) {
   //输出
   for v := range wc {
       fmt.Printf(v)
   }

}

func main() {

    r  := &ReadFilePath{
        path :"./access.log",
    }

    w  := &WriteDb{
        db :"username=liuli&password=liuli",
    }

    lp := &logprocess{

       //使用make来
       rc: make(chan []byte),
       wc: make(chan string),
       read:  r,
       write: w,

    }

    go lp.read.Read(lp.rc)
    go lp.Process()
    go lp.write.Write(lp.wc)

    //创建goroutine完后程序就自动退出  并不会等待
    time.Sleep(time.Second * 30)



}
