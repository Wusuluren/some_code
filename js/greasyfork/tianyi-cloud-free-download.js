// ==UserScript==
// @name           天翼云盘下载助手
// @author         wusuluren
// @description    天翼云盘免登录下载
// @require        http://cdn.bootcss.com/jquery/1.8.3/jquery.min.js
// @match          *://cloud.189.cn/*
// @supportURL     https://github.com/Wusuluren
// @version        0.0.1
// @grant          None
// @namespace https://greasyfork.org/users/194747
// ==/UserScript==
(function () {
    'use strict';
 
    $(function(){
        var timer = setInterval(function() {
          if(this.downloadUrl !== undefined) {
             $('.download-link').attr('href', this.downloadUrl)
             clearInterval(timer)
          }
        }, 100)
    })
})();
