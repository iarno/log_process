package main

import (
    "fmt"
    "time"
	"os"
	"log"
    "bufio"
    "io"
    "regexp"
    "strconv"
    "strings"
    "net/url"
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
    Write(wc chan *Message)
}

type logprocess struct {

   //channels进行通信
   rc chan []byte
   wc chan *Message

   //要读取日志的路径和influxdb的用户名、密码
   read  Reader
   write Writer

}

type Message struct {
    TimeLocal                    time.Time
    BytesSent                    int
    Path, Method, Scheme, Status string
    UpstreamTime, RequestTime    float64
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

    /**
        172.0.0.12 - - [04/Mar/2018:13:49:52 +0000] http "GET /foo?query=t HTTP/1.0" 200 2133 "-" "KeepAliveClient" "-" 1.005 1.854
    */

    r := regexp.MustCompile(`([\d\.]+)\s+([^ \[]+)\s+([^ \[]+)\s+\[([^\]]+)\]\s+([a-z]+)\s+\"([^"]+)\"\s+(\d{3})\s+(\d+)\s+\"([^"]+)\"\s+\"(.*?)\"\s+\"([\d\.-]+)\"\s+([\d\.-]+)\s+([\d\.-]+)`)

    //设置时区
    localTime, _ := time.LoadLocation("Asia/Shanghai")

    for v := range l.rc {

        ret := r.FindStringSubmatch(string(v))

        if len(ret) != 14 {

           log.Println("FindStringSubmach failed...")

        }

        message := &Message{}

        //转化为go中的时间格式
        t, err := time.ParseInLocation("02/Jan/2006:15:04:05 +0000", ret[4], localTime)

        if err != nil {
           log.Println("ParseInLocation fail:", err.Error(), ret[4])
           continue
        }

        message.TimeLocal = t
        byteSent, _ := strconv.Atoi(ret[8])
        message.BytesSent = byteSent

        // GET /foo?query=t HTTP/1.0
        reqSli := strings.Split(ret[6], " ")
        if len(reqSli) != 3 {
            log.Println("strings.Split fail", ret[6])
            continue
        }
        message.Method = reqSli[0]

        u, err := url.Parse(reqSli[1])
        if err != nil {
            log.Println("url parse fail:", err)
            continue
        }
        message.Path = u.Path

        message.Scheme = ret[5]
        message.Status = ret[7]

        upstreamTime, _ := strconv.ParseFloat(ret[12], 64)
        requestTime, _ := strconv.ParseFloat(ret[13], 64)
        message.UpstreamTime = upstreamTime
        message.RequestTime = requestTime

        l.wc <- message

    }


}

//3.写入influxdb中
func (w *WriteDb)  Write(wc chan *Message) {
   //输出
   for v := range wc {
       fmt.Println(v)
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
       wc: make(chan *Message),
       read:  r,
       write: w,

    }

    go lp.read.Read(lp.rc)
    go lp.Process()
    go lp.write.Write(lp.wc)

    //创建goroutine完后程序就自动退出  并不会等待
    time.Sleep(time.Second * 30)



}
