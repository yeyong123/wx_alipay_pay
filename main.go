/*
File Name: main.go
Created Date: 2022-04-18 19:51:19
Author: yeyong
Last modified: 2022-05-07 22:46:10
*/
package main

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "strconv"
    "strings"
    "time"
    "senkoo.cn/platform"
)

func main() {
    mux := http.NewServeMux()
    wx := platform.NewWxpay("", "", "")
    //wx.RequestGetCeriticate()
    wx.PaymentToUser()
    routers(mux)
    http.ListenAndServe(":9812", logRequest(mux))
}

func logRequest(target http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        target.ServeHTTP(w, r)
        requestIP := r.RemoteAddr
        r.ParseForm()
        str, err := json.Marshal(r.Form)
        if err != nil {
            fmt.Println(err)
        }
        log.Printf(
            "%s\t%s\t%s\t%s\t%v",
            r.Method,
            r.RequestURI,
            requestIP,
            str,
            time.Since(start),
        )
    })
}

func routers(mux *http.ServeMux) {
    mux.HandleFunc("/charge", ChargeHandler)
    mux.HandleFunc("/callback", CallbackHandler)
    mux.HandleFunc("/alipay_callback", AlipayHandler)
    mux.HandleFunc("/fetch_agent", FetchAgent)
}

func FetchAgent(w http.ResponseWriter, r *http.Request) {
    tmp, _ := json.Marshal(r.Header)
    platforms := map[string]string {
        "MicroMessenger": "wx",
        "AliApp": "alipay",
    }
    atmp := string(tmp)
    var agent string
    for k, v := range platforms {
        if strings.Contains(atmp, k) {
            agent = v
            break
        }
    }
    data := map[string]interface{} {
        "msg": "ok",
        "code": 200,
        "agent": agent,
    }
    json.NewEncoder(w).Encode(data)
}
func errMsg(w http.ResponseWriter, msg string) {
    data := map[string]interface{}{
        "msg": msg,
        "code": 422,
    }
    json.NewEncoder(w).Encode(data)
}

func ChargeHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    keys := map[string]string{
        "price": "",
        "body": "",
        "code": "",
        "platform": "",
    }
    plat := map[string]bool{ //支持的支付渠道
        "wx": true,
        "alipay": true,
    }

    for k, _ := range keys {
        tmp, ok := r.PostForm[k]
        if ok {
            keys[k] = tmp[0]
            if k == "body" && len(keys[k]) == 0 {
                keys[k] = "支付款项"
            }
        }
    }
    if len(keys["platform"]) == 0 {
        errMsg(w, "未选择支付渠道")
        return
    }
    if _, ok := plat[keys["platform"]]; !ok {
        errMsg(w, "目前只支持微信和支付宝支付")
        return
    }
    if keys["platform"] == "wx" && len(keys["code"]) == 0 {
        //微信支付检查是否有code 的值
        errMsg(w, "未获取的支付人个人信息")
        return
    }
    //检查传递的价格是否符合要求
    tmpCost, _ := strconv.Atoi(keys["price"])
    if tmpCost <= 0 {
        errMsg(w, "支付的金额无效")
        return
    }
    res, err := platform.GeneratePay(keys["platform"], keys["price"], keys["body"], keys["code"])
    if err != nil {
        errMsg(w, err.Error())
        return
    }
    data := map[string]interface{} {
        "msg": "ok",
        "code": 200,
        "charge": res,
    }
    json.NewEncoder(w).Encode(data)
}

func CallbackHandler(w http.ResponseWriter, r *http.Request) {
    defer func() {
        if err := recover(); err != nil {
            fmt.Println("Error", err)
            fmt.Fprint(w, "SUCCESS")
        }
    }()
    data, _ := ioutil.ReadAll(r.Body)
    var temp map[string]interface{}
    json.Unmarshal(data, &temp)
    if temp["event_type"].(string) == "TRANSACTION.SUCCESS" {
        tmp := temp["resource"].(map[string]interface{})
        text := tmp["ciphertext"].(string)
        nonce := tmp["nonce"].(string)
        tt, err := platform.DecryptToCallback(text, nonce)
        if err != nil {
            fmt.Fprint(w, "OK")
            return
        }
        amount := tt["amount"].(map[string]interface{})
        order_no := tt["out_trade_no"]
        created := tt["success_time"]
        msgParams := map[string]string{
            "platform": "微信支付",
            "order_no": order_no.(string),
            "body": "支付成功",
            "amount": fmt.Sprintf("%.2f", amount["total"].(float64)/ 100),
            "created": created.(string),
        }
        go sendNotify(msgParams)
    }
    fmt.Fprint(w, "OK")
}

func AlipayHandler(w http.ResponseWriter, r *http.Request) {
    r.ParseForm()
    keys := map[string]interface{} {
        "buyer_logon_id": "",
        "total_amount": "",
        "out_trade_no": "",
        "subject": "",
        "trade_status": "",
        "gmt_payment": "",
    }
    for k, _ := range keys {
        t, ok := r.PostForm[k]
        if ok {
            keys[k] = t[0]
        }
    }
    msgParams := map[string]string{
        "order_no": keys["out_trade_no"].(string),
        "platform": "支付宝支付",
        "body": keys["subject"].(string),
        "created": keys["gmt_payment"].(string),
        "amount": keys["total_amount"].(string),
    }
    go sendNotify(msgParams)
    fmt.Fprint(w, "SUCCESS")
}

func parseDate(stamp int) string {
    tm := time.Unix(int64(stamp), 0)
    y := tm.Year()
    m := tm.Month()
    d := tm.Day()
    h := tm.Hour()
    min := tm.Minute()
    sec := tm.Second()
    tamp := fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d", y, m, d, h, min, sec)
    return tamp
}

func sendNotify(pay map[string]string) error {
    msg := fmt.Sprintf("渠道: %s\n订单号: %s\n金额: %s\n支付款项: %s\n支付时间: %s", pay["platform"], pay["order_no"], pay["amount"], pay["body"], pay["created"])
    st, _ := json.Marshal(pay)
    write(string(st))
    fmt.Println(msg)
    return nil
}

func write(data string) {
    filePath := "payment.json"
    file, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND, 0666)
    if err != nil {
        fmt.Println("文件打开失败", err)
    }
    defer file.Close()
    write := bufio.NewWriter(file)
    write.WriteString(data + "\n")
    write.Flush()
}

type Payment struct {
    Id          string  `json:"id"`
    Object      string  `json:"object"`
    Livemode    string  `json:"livemode"`
    Paid        bool    `json:"paid"`
    Created     int     `json:"created"`
    App         string  `json:"app"`
    Channel     string  `json:"channel"`
    OrderNo     string  `json:"order_no"`
    Amount      int     `json:"amount"`
    Subject     string  `json:"subject"`
    Body        string  `json:"body"`
    Extra       Extra   `json:"extra"`
}

type Extra  struct {
    OpenId      string      `json:"open_id"`
}


