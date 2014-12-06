<?php

/**
 * 流包装器
 */
class ByfsStream
{
	const CODE_AUTH = 8888;

	const CODE_FILE_OPEN = 1;
	const CODE_FILE_READ = 2;
	const CODE_FILE_WRITE = 3;
	const CODE_FILE_LOCK = 4;
	const CODE_FILE_UNLOCK = 5;
	const CODE_FILE_SEEK = 6;
	const CODE_FILE_STAT = 7;
	const CODE_FILE_EOF = 8;
	const CODE_FILE_LUSH = 9;
	const CODE_FILE_TRUCATE = 10;
	const CODE_FILE_CLOSE = 11;

	const CODE_DIR_OPEN = 1001;
	const CODE_DIR_READ = 1002;
	const CODE_DIR_CLOSE = 1003;

	const CODE_MKDIR = 2001;
	const CODE_RMDIR = 2002;
	const CODE_COPY = 2003;
	const CODE_MOVE = 2004;
	const CODE_STAT = 2005;
	const CODE_LSTAT = 2006;

	private $fp;
	public $errno;

	public function connect($server, $port, $timeout=300, $auth='')
	{
		$req = array();
		$req[] = "POST / HTTP/1.1";
		$req[] = "Connection: Upgrade";
		$req[] = "Upgrade: Byfs-Stream";
		$req[] = "Byfs-Version: 1";
		//head空行
		$req[] = "\r\n";
		$req = implode("\r\n", $req);

		$fp = fsockopen($server, $port, $errno, $error, $timeout);
		if (!$fp) { return false; }

		//请求头
		$ok = fwrite($fp, $req);
		if ($ok !== strlen($req)) {
			fclose($fp);
			return false;
		}

		$head = array();
		while (($buf = fgets($fp, 2048)) !== false) {
			if ($buf == "\r\n") {
				break;
			}
			if (count($head) > 10) {
				fclose($fp);
				return false;
			}
		}

		if (!$head) {
			fclose($fp);
			return false;
		}

		//http协议升级
		if ($head[0] != "HTTP/1.1 101 Switching Protocols\r\n") {
			fclose($fp);
			return false;
		}

		if (!in_array("Connection: Upgrade\r\n", $head)) {
			fclose($fp);
			return false;
		}

		if (!in_array("Upgrade: Byfs-Stream\r\n", $head)) {
			fclose($fp);
			return false;
		}

		//认证
		foreach ($head as $tmp) {
			if (strpos($tmp, "Byfs-Auth:") === 0) {
				list($key, $val) = explode(':', $tmp, 2);
				$stream->write_uint16(ByfsStream::CODE_AUTH);
				$stream->write_string($this->_makeToken($val, $auth));
				$ok = $this->read_bool();
				if (!$ok) {
					fclose($fp);
					return false;
				}
				break;
			}
		}

		$this->fp = $fp;
		return true;
	}

	private function _makeToken($file, $auth)
	{
		$salt = dechex(mt_rand(0, 100000000));
		return md5($auth . $file . $salt) . $salt;
	}

	public function __destruct()
	{
		if ($this->fp) {
			fclose($this->fp);
		}
	}

	public function read_bool()
	{
		$num = $this->read_uint8();

		if ($num != 0) {
			$this->errno = $num;
			return false;
		}

		return true;
	}

	public function read_array_string()
	{
		$arr = array();
		$num = $this->read_uint16();

		while ($num > 0) {
			$arr[] = $this->read_string();
			$num--;
		}

		return $arr;
	}

	public function read_string()
	{
		$num = $this->read_uint32();

		return $this->_read($num);
	}

	public function write_string($data)
	{
		$this->write_uint32(strlen($data));

		return $this->_write($data);
	}

	###### 数字读取 #######

	public function read_uint8()
	{
		$data = $this->_read(1);
		$arr = unpack('C', $data);
		return $arr[0];
	}

	public function read_uint16()
	{
		$data = $this->_read(2);
		$arr = unpack('n', $data);
		return $arr[0];
	}

	public function read_uint32()
	{
		$data = $this->_read(4);
		$arr = unpack('N', $data);
		return $arr[0];
	}

	public function read_uint64()
	{
		$data = $this->_read(8);
		$arr = unpack('J', $data);
		return $arr[0];
	}

	###### 数字写入 #######

	public function write_uint8($number)
	{
		$data = unpack('C', $number);
		return $this->_write($data);
	}

	public function write_uint16($number)
	{
		$data = unpack('n', $number);
		return $this->_write($data);
	}

	public function write_uint32($number)
	{
		$data = unpack('N', $number);
		return $this->_write($data);
	}
	
	public function write_uint64($number)
	{
		$data = unpack('J', $number);
		return $this->_write($data);
	}

	####### 基础 #######

	private function _read($len)
	{
		$data = fread($this->fp, 1);

		if ($data === false) {
			throw new Exception('Stream Read Error!');
		}

		return $data;
	}

	private function _write($data)
	{
		$num = fwrite($this->fp, $data);

		if ($num === false) {
			throw new Exception('Stream Write Error!');
		}

		return $num;
	}

}

