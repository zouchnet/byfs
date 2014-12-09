<?php

/**
 * 流包装器
 */
class ByfsStream
{
	const CODE_AUTH = 8888;
	const CODE_CLOSE = 9999;

	const CODE_FILE_OPEN = 1;
	const CODE_FILE_READ = 2;
	const CODE_FILE_WRITE = 3;
	const CODE_FILE_LOCK = 4;
	const CODE_FILE_UNLOCK = 5;
	const CODE_FILE_SEEK = 6;
	const CODE_FILE_STAT = 7;
	const CODE_FILE_FLUSH = 8;
	const CODE_FILE_TRUCATE = 9;
	const CODE_FILE_CLOSE = 10;

	const CODE_DIR_OPEN = 1001;
	const CODE_DIR_READ = 1002;
	const CODE_DIR_CLOSE = 1003;

	const CODE_MKDIR = 2001;
	const CODE_RMDIR = 2002;
	const CODE_COPY = 2003;
	const CODE_STAT = 2004;
	const CODE_LSTAT = 2005;

	const O_RDONLY = 0x0;
	const O_WRONLY = 0x1;
	const O_RDWR = 0x2;
	const O_APPEND = 0x400;
	const O_CREATE = 0x40;
	const O_EXCL = 0x80;
	const O_SYNC = 0x101000;
	const O_TRUNC = 0x200;

	private $fp;
	public $errno;
	public $error;

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
			$head[] = $buf;
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
			$this->write_uint16(self::CODE_CLOSE);
			fclose($this->fp);
		}
	}

	public function read_bool()
	{
		$num = $this->read_uint8();

		if ($num != 0) {
			$this->errno = $num;
			$this->error = $this->read_string();
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
		$num = $this->read_uint16();

		if ($num == 0) {
			return '';
		}

		return $this->_read($num);
	}

	public function write_string($data)
	{
		$this->write_uint16(strlen($data));

		return $this->_write($data);
	}

	###### 数字读取 #######
	#
	public function read_int8()
	{
		$number = $this->read_uint8();
		return ($number > 127) ? ((-(127 - $number)) - 127 -2) : $number;
	}

	public function read_int16()
	{
		$number = $this->read_uint16();
		return ($number > 32767) ? ((-(32767 - $number)) - 32767 -2) : $number;
	}

	public function read_int32()
	{
		$number = $this->read_uint32();
		if (PHP_INT_MAX > 2147483647) {
			return ($number > 2147483647) ? ((-(2147483647 - $number)) - 2147483647 -2) : $number;
		}
		return $number;
	}

	public function read_int64()
	{
		$data = $this->_read(8);
		//php5.6才支持pack64位
		return $this->unpackInt64($data);
	}

	//-------------

	public function read_uint8()
	{
		$data = $this->_read(1);
		$arr = unpack('C', $data);
		return $arr[1];
	}

	public function read_uint16()
	{
		$data = $this->_read(2);
		$arr = unpack('n', $data);
		return $arr[1];
	}

	//32位上会变成有符号
	public function read_uint32()
	{
		$data = $this->_read(4);
		$arr = unpack('N', $data);
		return $arr[1];
	}

	//php无法支持int64
	public function read_uint64()
	{
		return $this->read_int64();
	}

	###### 数字写入 #######
	public function write_int8($number)
	{
		return $this->write_uint8($number);
	}

	public function write_int16($number)
	{
		return $this->write_uint16($number);
	}

	public function write_int32($number)
	{
		return $this->write_uint32($number);
	}

	public function write_int64($number)
	{
		return $this->write_uint64($number);
	}

	//------------
	
	public function write_uint8($number)
	{
		$data = pack('C', $number);
		return $this->_write($data);
	}

	public function write_uint16($number)
	{
		$data = pack('n', $number);
		return $this->_write($data);
	}

	public function write_uint32($number)
	{
		$data = pack('N', $number);
		return $this->_write($data);
	}
	
	public function write_uint64($number)
	{
		//php5.6才支持pack64位
		$data = $this->packInt64($number);
		return $this->_write($data);
	}

	####### 基础 #######

	private function _read($len)
	{
		$data = fread($this->fp, $len);

		if ($data === false) {
			throw new Exception('Stream Read Error!');
		}

		if (strlen($data) != $len) {
			throw new Exception('Stream Read EOF');
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

	private function packInt64($in)
	{
		$in = decbin($in);
		$in = str_pad($in, 64, '0', STR_PAD_LEFT);
		$out = '';
		for ($i = 0, $len = strlen($in); $i < $len; $i += 8) {
			$out .= chr(bindec(substr($in,$i,8)));
		}
		return $out;
	}

	private function unpackInt64($data)
	{
		$return = unpack('Nb/Na', $data);
		return $return['a'] + ($return['b'] << 32);
	}

}

