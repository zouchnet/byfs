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
	 * 下载远程文件到本地
	 */
	static public function get($path, $file)
	{
		$dst = fopen($file, 'wb');
		if (!$dst) { return false; }

		$src = self::_open($path, 'GET');
		if (!$src) {
			fclose($dst);
			return false;
		}

		stream_copy_to_stream($src, $dst);

		fclose($dst);
		fclose($src);
		
		return ($ok=='ok') ? true : false;
	}

	/**
	 * 上传本地文件到远程
	 */
	static public function put($file, $path)
	{
		$src = fopen($file, 'rb');
		if (!$src) { return false; }

		$ok = self::stream_put($src, $path);
		fclose($src);
		
		return $ok;
	}

	/**
	 * 删除远程文件
	 */
	static public function delete($path)
	{
		$fp = self::_open($path, 'DELETE');
		if (!$fp) { return false; }

		$ok = stream_get_contents($fp, 1024*4);
		fclose($fp);
		
		if ($ok == 'Success') {
			return true;
		}

		trigger_error("DELETE /{$file} Fail {$ok}");

		return false;
	}

	/**
	 * 测试一个文件是否存在
	 */
	static public function exists($path)
	{
		$src = self::_open($path, 'HEAD');
		if (!$src) { return false; }

		fclose($src);
		return true;
	}

	/**
	 * 打开一个读取流
	 */
	static public function stream_get($path)
	{
		return self::_open($path, 'GET');
	}

	/**
	 * 从流上传到文件
	 */
	static public function stream_put($src, $file)
	{
		if (!is_resource($src)) {
			return false;
		}
		return self::_put($src, $file);
	}

	/**
	 * 从数据上传到文件
	 */
	static public function data_put($data, $file)
	{
		if (!is_string($data)) {
			return false;
		}
		return self::_put($data, $file);
	}

	/**
	 * 打开连接
	 */
	static private function _open($file, $method)
	{
		if (strpos($file, 'byfs://') !== 0) {
			trigger_error('file protocol not support!', e_user_error);
			return false;
		}

		$file = substr($file, strlen('byfs://'));

		$headers = array();
		$headers[] = "byfs-version: 1";
		$headers[] = "connection: close";
		if ($method == 'DELETE') {
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
				'protocol_version' => version_compare(php_version, '5.3.0', '>=') ? 1.1 : 1.0,
				'timeout' => self::$timeout,
				'ignore_errors' => true,
			),
		);

		$ctx = stream_context_create($opts);

		$url = 'http://'.self::$server.":".self::$port.'/'. $file;

		$fp = fopen($url, 'rb', false, $ctx);
		if (!$fp) { return false; }

		$meta = stream_get_meta_data($src);

		$code = isset($meta['wrapper_data'][0]) ? $meta['wrapper_data'][0] : null;

		if ($code != 'HTTP/1.1 200 OK'&& $code != 'HTTP/1.0 200 OK') {
			fclose($fp);
			return false;
		}

		return $fp;
	}

	/**
	 * 上传文件
	 */
	static private function _put($src, $file)
	{
		if (strpos($file, 'byfs://') !== 0) {
			trigger_error('file protocol not support!', e_user_error);
			return false;
		}

		$file = substr($file, strlen('byfs://'));

		$req = array();
		$req[] = "PUT /{$file} HTTP/1.0";
		$req[] = "byfs-version: 1";
		$req[] = "content-type: application/octet-stream";
		if (self::$auth) {
			$salt = dechex(mt_rand(0, 100000000));
			$auth = md5(self::$auth . $file . $salt);
			$req[] = "auth: {$auth}{$salt}";
		}
		//head空行
		$req[] = "\r\n";

		$dst = fsockopen(self::$server, self::$port, $errno, $error, self::$timeout);
		if (!$dst) { return false; }

		$ok = fwrite($dst, implode("\r\n", $req));
		if ($ok === false) {
			fclose($dst);
			return false;
		}

		if (is_resource($src)) {
			stream_copy_to_stream($src, $dst);
		} else {
			$ok == fwrite($dst, $src);
			if ($ok === false) {
				fclose($dst);
				return false;
			}
		}

		//正常情况下服务器不会反回太长的数据
		$ok = stream_get_contents($dst, 1024*4);
		fclose($dst);
		if ($ok === false) {
			return false;
		}

		//过滤掉head,简易处理方式
		$ok = explode("\r\n", $ok);
		$msg = trim(array_pop($ok));
		unset($ok);

		var_dump($msg);
		if ($msg == "Success") {
			return true;
		}

		trigger_error("PUT /{$file} Fail {$msg}");

		return false;
	}

}


