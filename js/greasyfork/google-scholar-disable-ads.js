// ==UserScript==
// @name           谷歌学术镜像网站广告屏蔽助手
// @author         wusuluren
// @description    屏蔽谷歌学术镜像网站上的广告
// @require        http://cdn.bootcss.com/jquery/1.8.3/jquery.min.js
// @match          *://scholar.uulucky.com/*
// @match          *://scholar.fdwwr.cn/*
// @match          *://scholar.90h6.cn:1668/*
// @supportURL     https://github.com/Wusuluren
// @version        0.0.1
// @grant          None
// @namespace https://greasyfork.org/users/194747
// ==/UserScript==
(function () {
    'use strict';
    
    var timer1 = setInterval(function(){
      if($('#gs_res_ccl_bot > iframe:nth-child(4)')) {
          $('#gs_res_ccl_bot > iframe:nth-child(4)').remove();
          clearInterval(timer1)
       }   
    }, 10)
    var timer2 = setInterval(function(){
       if($('#gs_res_ccl_bot > iframe:nth-child(5)')) {
          $('#gs_res_ccl_bot > iframe:nth-child(5)').remove();
          clearInterval(timer2)
       }
    }, 10)
 
})();
