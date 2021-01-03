import requests, json, sqlite3, time


def load_json(filename="1.json"):
    with open(filename) as f:
        datas = json.loads(''.join(f.readlines()))
    print(datas)
    return datas


def main():
    load_config()

    headers = load_json("weibo-header.json")
    url = headers["referer"]
    resp = requests.get(url, headers=headers)
    # print(resp.text)
    posts = parseHtml(resp.text)
    # print(len(posts))
    # [print(post) for post in posts]
    checkNewPost(posts)


def checkNewPost(posts):
    conn = sqlite3.connect('weibo.db')
    c = conn.cursor()
    cursor = c.execute("SELECT id,text,ctime from post order by id desc limit 1")
    newPosts = []
    for row in cursor:
        # print(row)
        lastPost = row[1]
        for post in posts:
            if post != lastPost:
                newPosts.append(post)
            else:
                break

    ctime = int(time.time() * 1000)
    for post in newPosts[::-1]:
        sql = "insert into post (text,ctime) values ('%s',%d)" % (post, ctime)
        c.execute(sql)
        print(sql)
        notify(post)
        
    conn.commit()
    conn.close()


def parseHtml(html):
    try:
        posts = []
        xieyi = "<!--欧盟隐私协议弹窗-->"
        idx = html.index(xieyi)
        html = html[idx + len(xieyi):]

        fmViewBegin = "<script>FM.view("
        fmViewEnd = ")</script>"
        while True:
            idxBegin = html.index(fmViewBegin)
            idxEnd = html.index(fmViewEnd)
            # print(idxBegin, idxEnd)

            content = html[idxBegin + len(fmViewBegin):idxEnd]
            html = html[idxEnd + len(fmViewEnd):]
            # print(content)

            datas = json.loads(content)
            # print(datas.get("html", ""))
            texts = parseFmView(datas.get("html", ""))
            [posts.append(text) for text in texts if len(text) > 0]
    except ValueError:
        pass
    return posts


def parseFmView(html):
    try:
        texts = []
        divSplit = "<div action-data="
        divTextBegin = '<div class="WB_text W_f14"'
        divTextEnd = '</div>'
        while True:
            idxDiv = html.index(divSplit)
            html = html[idxDiv + len(divSplit):]
            # print(idxDiv)

            idxDivTextBegin = html.index(divTextBegin)
            idxDivTextEnd = html[idxDivTextBegin:].index(divTextEnd)
            # print(idxDivTextBegin, idxDivTextEnd)
            divText = html[idxDivTextBegin:idxDivTextBegin + idxDivTextEnd]
            # print(divText)
            text = getDivText(divText)
            if len(text) == 0:
                text = getDivImg(divText)
            # print(text)
            if len(text) > 0:
                texts.append(text)
            else:
                texts.append(divText.strip(" \r\n\t"))
                print("=========")
                print(divText)
                print(idxDivTextBegin, idxDivTextBegin + idxDivTextEnd)
                print("=========")
                # pass
    except ValueError:
        pass
    return texts


def getDivImg(html):
    try:
        imgBegin = "<img "
        title = 'title="'
        space = '" '
        text = ""
        while True:
            idxBegin = html.index(imgBegin)
            html = html[idxBegin + len(imgBegin):]
            idxTitle = html.index(title)
            idxSpace = html[idxTitle:].index(space)
            text += html[idxTitle + len(title):idxTitle + idxSpace]
    except ValueError:
        pass
    # print(text)
    text = text.strip(" \r\n\t")
    return text


def getDivText(html):
    try:
        kbegin = "<"
        kend = ">"
        text = ""
        while True:
            idxKBegin = html.index(kbegin)
            text += html[:idxKBegin]
            idxKEnd = html.index(kend)
            html = html[idxKEnd + len(kend):]
    except ValueError:
        text += html[idxKEnd:]
        pass
    # print(text)
    text = text.strip(" \r\n\t")
    return text


def notify(msg):
    token = get_token()
    send_msg(token, msg)


g = {}


def load_config(filename="wx-notify.json"):
    global g
    with open(filename) as f:
        g = json.loads(''.join(f.readlines()))
    print(g)


def get_token():
    global g
    return requests.get(
        "https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s" % (g['corpid'], g['corpsecret'])).json()[
        'access_token']


def send_msg(token, msg):
    global g
    data = {
        "touser": "@all",
        "msgtype": "text",
        "agentid": g['agentid'],
        "text": {
            "content": msg
        },
        "safe": 0
    }
    resp = requests.post("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s" % token, json=data)
    print(resp)


if __name__ == '__main__':
    main()
