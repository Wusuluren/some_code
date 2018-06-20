"""
微信通知接口
"""

import requests
import sys, json

g = {}

def load_config(filename="wx-notify.json"):
    global g
    with open(filename) as f:
        g = json.loads(''.join(f.readlines()))
    print(g)

def get_token():
    global g
    return requests.get("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s" % (g['corpid'], g['corpsecret'])).json()['access_token']

def send_msg(token, msg):
    global g
    data = {
           "touser" : "@all",
            "msgtype" : "text",
            "agentid" : g['agentid'],
            "text" : {
                "content" : msg
            },
            "safe":0
    }
    resp = requests.post("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s" % token, json=data)
    print(resp)


if __name__ == '__main__':
    if len(sys.argv) > 0:
        load_config()
        for msg in sys.argv[1:]:
            send_msg(get_token(), msg)
