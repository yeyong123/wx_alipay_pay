/*
File Name: alipay.go
Created Date: 2022-04-19 15:10:02
Author: yeyong
Last modified: 2022-05-07 20:46:30
*/
package platform

import (
	"fmt"
	"strconv"
	"github.com/smartwalle/alipay/v3"
)

type AlipayCharge struct {
    Total       string
    Desc        string
}

func NewAlipay(total, desc string) *AlipayCharge {
    return &AlipayCharge{
        Total: total,
        Desc: desc,
    }
}

func (ali *AlipayCharge) GeneratePay() (res map[string]string, err error) {
    client, err := alipay.New(AlipayAppid, AlipayPrivateKey, true)
    client.LoadAppPublicCertFromFile(AlipayAppPublicCertFile)
    client.LoadAliPayRootCertFromFile(AlipayRootCertFile)
    client.LoadAliPayPublicCertFromFile(AlipayRSAPublicCertFile)
    if err != nil {
        return res, err
    }
    cost_tmp, _ := strconv.Atoi(ali.Total)
    cost := fmt.Sprintf("%.2f", float64(cost_tmp) / 100)
    pay := alipay.TradeWapPay{}
    pay.NotifyURL = AlipayNotifyURL
    pay.ReturnURL = AlipayReturnURL
    pay.Subject = ali.Desc
    pay.OutTradeNo = GenOrderNo()
    pay.TotalAmount = cost
    pay.ProductCode = "QUICK_WAP_WAY"
    ret, err := client.TradeWapPay(pay)
    if err != nil {
        return res, err
    }
    res = map[string]string{
        "url": fmt.Sprintf("%v", ret),
    }
    return res, nil
}

func (ali *AlipayCharge) PaymentToUser() (result map[string]string, err error) {
    return result, nil
}
