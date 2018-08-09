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

    "github.com/influxdata/influxdb/client/v2"

    "flag"
    "net/http"
    "encoding/json"
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

// 系统状态监控
type SystemInfo struct {
    HandleLine   int     `json:"handleLine"`   // 总处理日志行数
    Tps          float64 `json:"tps"`          // 系统吞出量
    ReadChanLen  int     `json:"readChanLen"`  // read channel 长度
    WriteChanLen int     `json:"writeChanLen"` // write channel 长度
    RunTime      string  `json:"runTime"`      // 运行总时间
    ErrNum       int     `json:"errNum"`       // 错误数
}

const (
    TypeHandleLine = 0
    TypeErrNum     = 1
)

var TypeMonitorChan = make(chan int, 200)

type Monitor struct {
    startTime time.Time
    data      SystemInfo
    tpsSli    []int
}

func (m *Monitor) start(lp *logprocess) {

    go func() {
        for n := range TypeMonitorChan {
            switch n {
            case TypeErrNum:
                m.data.ErrNum += 1
            case TypeHandleLine:
                m.data.HandleLine += 1
            }
        }
    }()

    ticker := time.NewTicker(time.Second * 5)
    go func() {
        for {
            <-ticker.C
            m.tpsSli = append(m.tpsSli, m.data.HandleLine)
            if len(m.tpsSli) > 2 {
                m.tpsSli = m.tpsSli[1:]
            }
        }
    }()

    http.HandleFunc("/monitor", func(writer http.ResponseWriter, request *http.Request) {
        m.data.RunTime = time.Now().Sub(m.startTime).String()
        m.data.ReadChanLen = len(lp.rc)
        m.data.WriteChanLen = len(lp.wc)

        if len(m.tpsSli) >= 2 {
            m.data.Tps = float64(m.tpsSli[1]-m.tpsSli[0]) / 5
        }

        ret, _ := json.MarshalIndent(m.data, "", "\t")
        io.WriteString(writer, string(ret))
    })

    http.ListenAndServe(":9193", nil)
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
       TypeMonitorChan <- TypeHandleLine
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
           TypeMonitorChan <- TypeErrNum
           continue
        }

        message.TimeLocal = t
        byteSent, _ := strconv.Atoi(ret[8])
        message.BytesSent = byteSent

        // GET /foo?query=t HTTP/1.0
        reqSli := strings.Split(ret[6], " ")
        if len(reqSli) != 3 {
            TypeMonitorChan <- TypeErrNum
            log.Println("strings.Split fail", ret[6])
            continue
        }
        message.Method = reqSli[0]

        u, err := url.Parse(reqSli[1])
        if err != nil {
            log.Println("url parse fail:", err)
            TypeMonitorChan <- TypeErrNum
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

    // 写入模块

    infSli := strings.Split(w.db, "@")

    // Create a new HTTPClient
    c, err := client.NewHTTPClient(client.HTTPConfig{
        Addr:     infSli[0],
        Username: infSli[1],
        Password: infSli[2],
    })
    if err != nil {
        log.Fatal(err)
    }

    for v := range wc {
        // Create a new point batch
        bp, err := client.NewBatchPoints(client.BatchPointsConfig{
            Database:  infSli[3],
            Precision: infSli[4],
        })
        if err != nil {
            log.Fatal(err)
        }

        // Create a point and add to batch
        // Tags: Path, Method, Scheme, Status
        tags := map[string]string{"Path": v.Path, "Method": v.Method, "Scheme": v.Scheme, "Status": v.Status}
        // Fields: UpstreamTime, RequestTime, BytesSent
        fields := map[string]interface{}{
            "UpstreamTime": v.UpstreamTime,
            "RequestTime":  v.RequestTime,
            "BytesSent":    v.BytesSent,
        }

        pt, err := client.NewPoint("mydb", tags, fields, v.TimeLocal)
        if err != nil {
            log.Fatal(err)
        }
        bp.AddPoint(pt)

        // Write the batch
        log.Println(bp)
        if err := c.Write(bp); err != nil {
            log.Fatal(err)
        }

        log.Println("write success!")
    }

}

func main() {
    var path, influxDsn string
    flag.StringVar(&path, "path", "./access.log", "read file path")
    flag.StringVar(&influxDsn, "influxDsn", "http://127.0.0.1:8086@liuli@liuli@mydb@s", "influx data source")
    flag.Parse()

    r  := &ReadFilePath{
        path : path,
    }

    w  := &WriteDb{
        db : influxDsn,
    }

    lp := &logprocess{

       //使用make来
       rc: make(chan []byte),
       wc: make(chan *Message),
       read:  r,
       write: w,

    }

    go lp.read.Read(lp.rc)
    for i := 0; i < 2; i++ {
        go lp.Process()
    }

    for i := 0; i < 4; i++ {
        go lp.write.Write(lp.wc)
    }

    m := &Monitor{
        startTime: time.Now(),
        data:      SystemInfo{},
    }
    m.start(lp)



}
