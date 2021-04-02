CREATE TABLE `test1` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT COMMENT '',
  `age` bigint(20) DEFAULT '0' COMMENT '',
  `name` varchar(16) COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT '',
  `status` tinyint(4) NOT NULL DEFAULT '0' COMMENT '',
  `ctime` bigint(20) NOT NULL DEFAULT '0' COMMENT '',
  `utime` bigint(20) NOT NULL DEFAULT '0' COMMENT '',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=1 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci COMMENT='';

