package main
import (
    "io/ioutil"
    "regexp"
    "net/http"
    "net/url"
    "bytes"
    "time"
    "sort"
    "fmt"
)

type urlsPool struct {
    data [][]byte 				//静池
    messy [][][]byte 			//脏池
    pos int 					//浮标
    err []byte 					//错误信息
}
func (usp *urlsPool) adderMessy(v [][][]byte) { usp.messy=append(usp.messy,v...) }
func (usp *urlsPool) errInfo(v []byte) { usp.err=append(usp.err,v...)}
func (usp *urlsPool) Clear() {
	//去掉脏池重复项及补全URL
	messyData:=uniqueResave(usp.messy)
	//清洗脏池
	usp.messy=[][][]byte{}
	//去掉脏池重复静池的值
	newData:=uniqueResaveMany(usp.data,messyData)
	//浮标置顶
	usp.pos=len(usp.data)
	//添加新数据到静池
	usp.data=append(usp.data,newData...)
}
var (
    ch chan [][][]byte                //管道
    deep int                     //深度
    deepOut int
    domain string                //域名
    timeout string               //超时
    filename string 			 //生成的文件,路径+文件名
)

var pooler = &urlsPool{}             //池子
var client http.Client       	 //http客户端


func main() {
    //任务设置
    //要访问的域名，即将遍历的网站,这个网站的任意一个URL都可以
    domain="http://www.saiaodi.cn"

    //访问网络的超时，默认5秒 。
    //从开始发起请求开始计时，5秒之后，无论网络处于何种状态都将断开连接，即使是正在接收。
    timeout="5s"

    //深度 ，任务的批次，默认1000，不是URL的总数。
    //每完成对一批URL的访问深度+1
    deepOut = 1000

    //文件名
    //路径+文件名
    f,_:=url.Parse(domain)
    filename = "./"+f.Host+".txt"
    // filename = "./sitemap.txt"

	//管道缓存,默认200
	//go的管道缓存数量?
	ch = make(chan [][][]byte,200)
	client.Timeout,_=time.ParseDuration(timeout)

	/**************************** 完成配置 **************************/
    
    //启动流程
    body,_:=Visit([]byte(domain))
    pooler.adderMessy(pickUrl(body))
    pooler.Clear()
    routine(ch)
    file:=uSort(pooler.data)
    ioutil.WriteFile(filename,file,0666)
    ioutil.WriteFile(filename+".err",pooler.err,0666)
    fmt.Println("已完成")
}
func routine(ch chan [][][]byte) bool{
	//工作主程序
    for i:=pooler.pos; i<len(pooler.data); i++ {
        go func(url []byte) {
            defer func(){
                if e:=recover(); e!=nil {
                    fmt.Println("go协程错误报告：",e)
                }
            }()
            body,status:=Visit(url)
            if !status {pooler.errInfo(body)}
            ch<-pickUrl(body)
        }(pooler.data[i])
    }
    for i:=pooler.pos; i<len(pooler.data); i++ {
        pooler.adderMessy(<-ch)
    }
    //清洗脏池
    pooler.Clear()
    //计算深度
    deep++
    if deep>= deepOut {return true}
    if len(pooler.data) > pooler.pos {
        routine(ch)
    }
    return false
}
func Visit(u []byte) (body []byte,_ bool) {
	//网络访问
    defer func() {
        if e:=recover(); e!=nil {
            fmt.Println("visit访问网络错误报告:",e)
        }
    }()
    resp,_:=client.Get(string(u))
    if resp.StatusCode == 200 {
        body,_=ioutil.ReadAll(resp.Body)
        return body,true
    }
    resp.Body.Close()
    //组织错误信息
    errstr:=[]byte(resp.Status+"  ")
    errstr=append(errstr,u...)
    errstr=append(errstr,[]byte("\n")...)
    return errstr,false
}
func pickUrl(p []byte) (urls [][][]byte){
	//筛选URL
    p= regexp.MustCompile(`\s`).ReplaceAll(p,[]byte{})
    urls = regexp.MustCompile(`<a.*?href=\"(.*?)\".*?>`).FindAllSubmatch(p,-1)
    return urls
}

func uniqueResaveMany(f [][]byte,s [][]byte) (ns [][]byte){
	//有参照数组去重
    for m:=0; m < len(f); m++ {
        for n:=0; n<len(s);n++ {
            if bytes.Equal(f[m],s[n]) { s[n]=[]byte{} }
        }
    }
    for i:=0;i<len(s);i++ {
        if len(s[i])!=0 { ns=append(ns,s[i]) }
    }
    return ns
}
func uniqueResave(s [][][]byte) (ns [][]byte){
	//数组自去重
    for m:=0; m<len(s); m++ {
        for n:=m+1; n<len(s); n++ {
            if bytes.Equal(s[m][1],s[n][1]) {
                s[n][1]=nil
            }
        }
        if s[m][1] !=nil {
        	urlTmp:=urlValidate(s[m][1]);
        	if urlTmp != nil {
            	ns=append(ns,urlTmp)
        	}
        }
    }
    return ns
}
func uSort(s [][]byte) []byte{
	//排序
    ns :=[]string{} 
    for i := 0; i < len(s); i++ {
        ns=append(ns,string(s[i]))
    }
    sort.Strings(ns)
    ss:=[]byte{}
    for i := 0; i < len(ns); i++ {
        ss=append(ss,[]byte(ns[i]+"\n")...)
    }
    return ss
}

func urlValidate(u []byte) []byte{
	//URL补全验证
	d,_:=url.Parse(domain)
	n,_:=url.Parse(string(u))
	//如果是相对路径,补全域名
	if len(u)>0 && u[0] == 47 { return append([]byte(d.Scheme+"://"+d.Host),u...) }
	//对非本域或空值,返回空
	if d.Host!=n.Host || len(u)== 0 { return []byte{} }

	return u
}
