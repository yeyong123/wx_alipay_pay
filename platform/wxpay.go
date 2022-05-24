/*
File Name: weixin_pay.go
Created Date: 2022-04-19 19:53:47
Author: yeyong
Last modified: 2022-05-24 11:20:45
*/
package platform

import (
    "context"
    "log"
    "net/http"
    "time"
    "fmt"
    "errors"
    "strconv"
    "encoding/json"
    "crypto/rsa"
    "io/ioutil"
    "math/rand"
    "crypto/tls"
    "github.com/wechatpay-apiv3/wechatpay-go/core"
    "github.com/wechatpay-apiv3/wechatpay-go/core/option"
    "github.com/wechatpay-apiv3/wechatpay-go/utils"
    "github.com/wechatpay-apiv3/wechatpay-go/services/transferbatch"
)

type WechatPay struct {
    ctx  context.Context
    privateKey *rsa.PrivateKey
    client  *core.Client
    appid   string
    Total   string
    Desc    string
    Code    string
    UserList map[string]string
}


func NewWxpay(total, desc, code string) *WechatPay {
    privateKey, err := utils.LoadPrivateKey(WxPrivateKey)
    if err != nil {
        log.Println("解密错误", err)
        return nil
    }
    options := []core.ClientOption{
        option.WithWechatPayAutoAuthCipher(MchID, SerialNumber, privateKey, WxV3Key),
        //option.WithoutValidator(),
        option.WithHTTPClient(&http.Client{
            Transport: &http.Transport{
                TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
            },
        }),
    }
    ctx := context.Background()
    client, err := core.NewClient(ctx, options...)
    if err != nil {
        log.Println("生成微信客户端错误", err)
        return nil
    }
    return &WechatPay{
        ctx: ctx,
        privateKey: privateKey,
        client: client,
        appid: WxAppID,
        Total: total,
        Desc: desc,
        Code: code,
    }
}

func DecryptToCallback(text, nonce string) (data map[string]interface{}, err error) {
    //回调的接口, 解码微信支付后的数据
    s, err := utils.DecryptAES256GCM(WxV3Key, "transaction", nonce, text)
    if err != nil {
        log.Println("解码失败", err)
        return data, err
    }
    json.Unmarshal([]byte(s), &data)
    if err != nil {
        return data, err
    }
    return data, nil
}
func (pay *WechatPay) GeneratePay() (result map[string]string, err error) {
    //生成付款请求
    oid, err := GetWechatOpenId(pay.Code)
    if err != nil {
        return nil, errors.New("未获取到用户信息")
    }
    cost, _ := strconv.Atoi(pay.Total)
    data := map[string]interface{}{
        "mchid": MchID,
        "out_trade_no": GenOrderNo(),
        "appid": WxAppID,
        "description": pay.Desc,
        "notify_url": WxNotifyURL,
        "amount": map[string]interface{}{
            "total": cost,
            "currency": "CNY",
        },
        "payer": map[string]interface{}{
            "openid": oid["openid"],
        },
    }
    url := "https://api.mch.weixin.qq.com/v3/pay/transactions/jsapi"
    resp, err := pay.client.Post(pay.ctx, url, data)
    if err != nil {
        d, _ := json.Marshal(err)
        var errMap map[string]interface{}
        json.Unmarshal(d, &errMap)
        log.Println("发送支付请求失败", errMap)
        return result, errors.New(errMap["message"].(string))
    }
    body, _ := ioutil.ReadAll(resp.Response.Body)
    json.Unmarshal(body, &result)
    pack := fmt.Sprintf("prepay_id=%s", result["prepay_id"])
    result = pay.generateSign(pack)
    return result, nil
}

func (pay *WechatPay) PaymentToUser() (result map[string]string, err error) {
    //付款到零钱
    svc := transferbatch.TransferBatchApiService{Client: pay.client}
    resp, res, err := svc.InitiateBatchTransfer(pay.ctx,
		transferbatch.InitiateBatchTransferRequest{
			Appid:       core.String(WxAppID),
			BatchName:   core.String("2019年1月深圳分部报销单"),
			BatchRemark: core.String("2019年1月深圳分部报销单"),
			OutBatchNo:  core.String(GenOrderNo()),
			TotalAmount: core.Int64(200),
			TotalNum:    core.Int64(1),
			TransferDetailList: []transferbatch.TransferDetailInput{transferbatch.TransferDetailInput{
				TransferAmount:     core.Int64(200),
				UserName:           core.String("xxx"),
				OutDetailNo:        core.String(GenOrderNo()),
				TransferRemark:     core.String("2020年4月报销"),
				Openid:             core.String("xxxx"),
			}},
		},
	)
    if err != nil {
        fmt.Println("发起付款失败: ", err)
        return nil, err
    }
    fmt.Println("发起付款成功: ", res.Response.StatusCode, resp)
    return result, nil
}

func (pay *WechatPay) randStr() string {
    var letters = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
    rand.Seed(time.Now().UnixNano())
    b := make([]rune, 32)
    for i := range b {
        b[i] = letters[rand.Intn(len(letters))]
    }
    return string(b)
}
func (pay *WechatPay) generateSign(key string) map[string]string {
    var result = make(map[string]string)
    dtime := time.Now().Unix()
    nonce := pay.randStr()
    sign := fmt.Sprintf("%s\n%d\n%s\n%s\n",pay.appid, dtime, nonce, key)
    res, err := utils.SignSHA256WithRSA(sign, pay.privateKey)
    if err != nil {
        log.Println("签名失败", err)
        return result
    }
    result["timeStamp"] = fmt.Sprintf("%d", dtime)
    result["nonceStr"] = nonce
    result["package"] = key
    result["signType"] = "RSA"
    result["paySign"] = res
    result["appId"] = pay.appid
    return result
}
func (pay *WechatPay) RequestGetCeriticate() error {
    /*
    操作请求证书使用的业务
    */
    URL := "https://api.mch.weixin.qq.com/v3/certificates"
    res, err := pay.client.Get(pay.ctx, URL)
    if err != nil {
        log.Println("出现了错误: ", err)
        return err
    }
    body, err := ioutil.ReadAll(res.Response.Body)
    if err != nil {
        log.Println("错误: ", err)
        return err
    }
    var data = make(map[string][]interface{})
    json.Unmarshal(body, &data)
    key := data["data"][0].(map[string]interface{})["encrypt_certificate"].(map[string]interface{})
    text := key["ciphertext"].(string)
    nonce := key["nonce"].(string)
    cate := key["associated_data"].(string)
    unKey, err := utils.DecryptAES256GCM(WxV3Key, cate, nonce, text)
    if err != nil {
        log.Println("解密证书失败", err)
        return err
    }
    fmt.Println(unKey)
    return nil
}

func GenOrderNo() string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
    k := r.Intn(999999999999999)
    return fmt.Sprintf("SKN%d", k)
}



func GetWechatOpenId(code string) (res map[string]string, err error) {
    //根据 code 获取用户的 openID
     params := fmt.Sprintf("?appid=%s&secret=%s&code=%s&grant_type=authorization_code", WxAppID, WxAppSecret, code)
    url := fmt.Sprintf("https://api.weixin.qq.com/sns/oauth2/access_token%s", params)
    tr := &http.Transport{
        TLSClientConfig:    &tls.Config{InsecureSkipVerify: true},
    }
    client := &http.Client{Transport: tr}

    resp, err := client.Get(url)
    if err != nil {
        return res, err
    }
    defer resp.Body.Close()
    data, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return res, err
    }
    json.Unmarshal(data, &res)
    return res, nil
}


