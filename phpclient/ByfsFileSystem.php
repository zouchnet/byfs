<?php

/**
 * byfs文件系统
 */
class ByfsFileSystem
{
	static private $server;
	static private $port;
	static private $timeout;
	static private $auth;

	static private $stream;

	static public function connect()
	{
		if (!self::$stream) {
			self::$stream = new ByfsStream();
			$ok = self::$stream->connect(self::$server, self::$port, self::$timeout, self::$auth);
			if (!$ok) {
				throw new Exception('byfs connect error');
			}
		}

		return self::$stream;
	}

	static public function close()
	{
		self::$stream = null;
	}

	static public function mkdir($path, $mode = 0777, $recursive=false)
	{
		$stream = self::connect();

		$stream->write_uint16(ByfsStream::CODE_MKDIR);
		$stream->write_string($path);
		$stream->write_uInt16($mode);
		$stream->write_uInt8($recursive);

		return $stream->read_bool();
	}

	static public function rename($from, $to)
	{
		$stream = self::connect();

		$stream->write_uint16(ByfsStream::CODE_MOVE);
		$stream->write_string($from);
		$stream->write_string($to);

		return $stream->read_bool();
	}

	static public function rmdir($path, $recursive=false)
	{
		$stream = self::connect();

		$stream->write_uint16(ByfsStream::CODE_RMDIR);
		$stream->write_string($path);
		$stream->write_uInt8($recursive ? 1 : 0);

		return $stream->read_bool();
	}

	static public function unlink($path)
	{
		$stream = self::connect();

		$stream->write_uint16(ByfsStream::CODE_RMDIR);
		$stream->write_string($path);

		return $stream->read_bool();
	}

	static public function stat($path)
	{
		$stream = self::connect();

		$stream->write_uint16(ByfsStream::CODE_STAT);
		$stream->write_string($path);

		return $stream->read_string();
	}

	static public function lstat($path)
	{
		$stream = self::connect();

		$stream->write_uint16(ByfsStream::CODE_LSTAT);
		$stream->write_string($path);

		return $stream->read_string();
	}
   
    static public function fopen($path, $mode, $options, $opened_path)
    {
		$stream = self::connect();

		$file = new ByfsStreamFile($stream);
		$ok = $file->open($path, $mode);

		return $ok ? $file : false;
    }

	static public function opendir($path)
	{
		$stream = self::connect();

		$file = new ByfsStreamDir($stream);
		$ok = $file->open($path);

		return $ok ? $file : false;
	}

}

