// ==UserScript==
// @name           谷歌镜像网站广告屏蔽助手
// @author         wusuluren
// @description    屏蔽谷歌镜像网站上的广告
// @require        http://cdn.bootcss.com/jquery/1.8.3/jquery.min.js
// @match          *://google.uulucky.com/*
// @match          *://google.gccpw.cn/*
// @match          *://google.90h6.cn:1668/*
// @supportURL     https://github.com/Wusuluren
// @version        0.0.1
// @grant          None
// @namespace https://greasyfork.org/users/194747
// ==/UserScript==
(function () {
    'use strict';
    
    setInterval(function(){
      if(document.querySelector('#center_col > div:nth-child(5) > div:nth-child(1)')) {
          $('#center_col > div:nth-child(5) > div:nth-child(1)').remove();
          clearInterval()
       }   
    }, 10)
    setInterval(function(){
       if(document.querySelector('#rhs_block')) {
          $("#rhs_block").remove();
          clearInterval()
       }
    }, 10)
 
})();