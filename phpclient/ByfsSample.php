<?php

/**
 * 简易分布式文件系统PHP客户端
 * 这个是标准HTTP协议模式
 * PHP对HTTP支持最好，推荐使用这种方式
 */
class Byfs
{
	static private $server;
	static private $port;
	static private $timeout;
	static private $auth;

	/**
	 * 设置配置
	 */
	static public function init($server, $port, $timeout, $auth)
	{
		self::$server = $server;
		self::$port = $port;
		self::$timeout = $timeout;
		self::$auth = $auth;
	}

	/**
	 * 测试一个文件是否存在
	 */
	static public function exists($path)
	{
		$src = self::_open($path, 'HEAD');
		if (!$src) { return false; }

		$ok = stream_get_contents($dst);
		fclose($src);
		
		return ($ok=='ok') ? true : false;
	}

	/**
	 * 打开一个读取流
	 */
	static public function stream_get($path)
	{
		return self::_open($path, 'GET');
	}

	/**
	 * 打开一个写入流
	 */
	static public function stream_put($path)
	{
		return self::_open($path, 'PUT');
	}

	/**
	 * 下载远程文件到本地
	 */
	static public function get($path, $file)
	{
		$dst = fopen($file, 'wb');
		if (!$dst) { return false; }

		$src = self::_open($path, 'GET');
		if (!$src) { return false; }

		stream_copy_to_stream($src, $dst);
		$ok = stream_get_contents($dst);
		fclose($dst);
		
		return ($ok=='ok') ? true : false;
	}

	/**
	 * 上传本地文件到远程
	 */
	static public function put($file, $path)
	{
		$src = fopen($file, 'rb');
		if (!$src) { return false; }

		$dst = self::_open($path, 'GET');
		if (!$dst) { return false; }

		stream_copy_to_stream($src, $dst);
		$ok = stream_get_contents($dst);
		fclose($dst);
		
		return ($ok=='ok') ? true : false;
	}

	/**
	 * 删除远程文件
	 */
	static public function delete($path)
	{
		$fp = self::_open($path, 'DELETE');
		if (!$fp) { return false; }

		$ok = stream_get_contents($fp);
		fclose($fp);
		
		return ($ok=='ok') ? true : false;
	}

	/**
	 * 打开连接
	 */
	static private function _open($file, $method)
	{
		if (strpos($file, 'byfs://') !== 0) {
			trigger_error('file protocol not support!', E_USER_ERROR);
			return false;
		}

		$file = substr($file, strlen('byfs://'));

		$headers = array();
		$headers[] = "byfs: 1";
		$headers[] = "Connection: close";
		if ($method != 'GET') {
			$headers[] = "Content-Type: application/octet-stream";
			if (self::$auth) {
				$salt = dechex(mt_rand(0, 100000000));
				$auth = md5(self::$auth . $file . $salt);
				$headers[] = "auth: {$auth}{$salt}";
			}
		}

		$opts = array(
			'http' => array(
				'method' => $method,
				'header' => implode("\r\n", $headers),
				'protocol_version' => version_compare(PHP_VERSION, '5.3.0', '>=') ? 1.1 : 1.0,
				'timeout' => self::$timeout,
				'ignore_errors' => true,
			),
		);

		$ctx = stream_context_create($opts);

		$url = 'http://'.self::$server.":".self::$port.'/'. $file;

		return fopen($url, 'rb', false, $ctx);
	}


}


