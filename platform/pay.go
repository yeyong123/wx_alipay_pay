/*
File Name: pay.go
Created Date: 2022-04-20 09:52:55
Author: yeyong
Last modified: 2022-05-24 11:20:00
*/
package platform

import (
    "errors"
)
const (
    WxPrivateKey = `-----BEGIN PRIVATE KEY-----
-----END PRIVATE KEY-----`

WxCertificate =`-----BEGIN CERTIFICATE-----
-----END CERTIFICATE-----`
    AlipayPrivateKey=``
    SerialNumber = ""
    MchID = ""
    WxAppID = ""
    WxAppSecret = ""
    WxV3Key = ""
    WxNotifyURL = "https://xxx.com/callback"
    AlipayAppid = ""
    AlipayAppPublicCertFile = "cert/alipay/appCertPublicKey_2019102168503764.crt"
    AlipayRootCertFile = "cert/alipay/alipayRootCert.crt"
    AlipayRSAPublicCertFile = "cert/alipay/alipayCertPublicKey_RSA2.crt"
    AlipayNotifyURL = "https://xxx.com/alipay_callback"
    AlipayReturnURL = "https://xxx.com/result"
)


type Payment interface{
    GeneratePay() (map[string]string, error)
    PaymentToUser()(map[string]string, error)
}

func NewPayment(plat, total, desc, code string) Payment {
    var pay Payment
    if plat == "wx" {
        p := NewWxpay(total, desc, code)
        pay = p
    } else if (plat == "alipay") {
        p := NewAlipay(total, desc)
        pay = p
    }
    return pay
}

func GeneratePay(plat, total, desc, code string) (res map[string]string, err error) {
    pay := NewPayment(plat, total, desc, code)
    if pay == nil {
        return nil, errors.New("所选择的渠道无效")
    }
    return pay.GeneratePay()
}
