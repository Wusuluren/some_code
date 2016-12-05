#!/usr/bin/env python
#coding=utf8

#Download images from baidu tieba

import urllib
import re
import sys

def GetHtmlPageNum(url):
    html = urllib.urlopen(url)
    page = html.read()
    pagesRe = re.compile(r'共<span class=".+?">(.+?)</span>页</li>')
    pageNum = re.findall(pagesRe, page)
    return max(pageNum)

def GetHtml(url):
    html = urllib.urlopen(url)
    page = html.read()
    return page

def DownloadImg(page, pageNum):
    imgRe = re.compile(r'src="(http://imgsrc.baidu.com/forum/.*?\.jpg)"')
    imgUrls = re.findall(imgRe, page)
    fileNo = 1
    for i in imgUrls:
        print '%s:%s' % (fileNo, i)
        urllib.urlretrieve(i, 'page-%s-%s.jpg' % (pageNum, fileNo))
        fileNo += 1

if __name__ == '__main__':
    for arg in sys.argv[1:]:
        pageNum = GetHtmlPageNum(arg)
        print 'totoal pages:%s' % pageNum
        for i in xrange(int(pageNum)):
            pageNo = i + 1
            pageUrl = arg + r'?pn=%s' % pageNo
            print 'downloading:%s' % pageUrl
            DownloadImg(GetHtml(pageUrl), pageNo)
        print '%s:all pages download done' % arg
