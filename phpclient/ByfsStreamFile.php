<?php

/**
 * 文件操作协议
 */
class ByfsStreamFile
{
	private $stream;
	private $fp;
	private $offset;
	private $eof;

	public function __construct(ByfsStream $stream)
	{
		$this->stream = $stream;
	}

    public function open($path, $mode)
    {
		$mode = strtolower(trim(trim($mode), 'b'));

		$flag = 0;
		switch ($mode) {
		case 'r' :
			$flag = ByfsStream::O_RDONLY;
			break;
		case 'r+' :
			$flag = ByfsStream::O_RDWR;
			break;
		case 'w' :
			$flag = ByfsStream::O_WRONLY | ByfsStream::O_TRUNC | ByfsStream::O_CREATE;
			break;
		case 'w+' :
			$flag = ByfsStream::O_RDWR | ByfsStream::O_TRUNC | ByfsStream::O_CREATE;
			break;
		case 'a' :
			$flag = ByfsStream::O_APPEND | ByfsStream::O_CREATE;
			break;
		case 'a+' :
			$flag = ByfsStream::O_RDWR | ByfsStream::O_CREATE;
			break;
		case 'x' :
			$flag = ByfsStream::O_WRONLY | ByfsStream::O_CREATE | ByfsStream::O_EXCL;
			break;
		case 'x+' :
			$flag = ByfsStream::O_RDWR | ByfsStream::O_CREATE | ByfsStream::O_EXCL;
			break;
		case 'c' :
			$flag = ByfsStream::O_WRONLY | ByfsStream::O_CREATE;
			break;
		case 'c+' :
			$flag = ByfsStream::O_RDWR | ByfsStream::O_CREATE;
			break;
		default:
			throw new Exception("未识别的文件打开模式:{$mode}");
		}

		$this->stream->write_uint16(ByfsStream::CODE_FILE_OPEN);
		$this->stream->write_string($path);
		$this->stream->write_int32($flag);

		$ok = $this->stream->read_bool();
		if (!$ok) {
			return false;
		}

		$this->fp = $this->stream->read_uint32();

		if ($this->fp) {
			if ($mode == 'a+') {
				$this->seek(SEEK_END,0);
			}
		}

		return $this->fp != 0 ? true : false;
    }

	public function __destruct()
	{
		$this->close();
	}
	
	public function close()
	{
		if ($this->fp) {
			if (!$this->stream->fp) { return false; }

			$this->stream->write_uint16(ByfsStream::CODE_FILE_CLOSE);
			$this->stream->write_uint32($this->fp);
			$this->fp = null;
			return $this->stream->read_bool();
		}
	}

	public function read($count)
    {
		$this->offset = null;
		$this->stream->write_uint16(ByfsStream::CODE_FILE_READ);
		$this->stream->write_uint32($this->fp);
		$this->stream->write_int64($count);
		$ok = $this->stream->read_bool();
		if (!$ok) {
			return false;
		}

		$data = "";

		do {
			$tmp = $this->stream->read_string();
			$data .= $tmp;
		} while ($tmp != "");

		$ok = $this->stream->read_bool();
		if (!$ok) {
			return false;
		}

		if (strlen($data) < $count) {
			$this->eof = true;
		}

		return $data;
    }

    public function write($data)
	{
		$this->offset = null;
		$this->stream->write_uint16(ByfsStream::CODE_FILE_WRITE);
		$this->stream->write_uint32($this->fp);

		$len = strlen($data);

		$data = str_split($data, 4096);
		foreach ($data as $tmp) {
			$this->stream->write_string($tmp);
		}
		$this->stream->write_uint16(0);

		$ok = $this->stream->read_bool();
		if (!$ok) { return false; }

		return $len;
    }

	public function eof()
    {
		return $this->eof;
    }

	public function flush()
	{
		if (!$this->stream->fp) { return false; }

		$this->stream->write_uint16(ByfsStream::CODE_FILE_FLUSH);
		$this->stream->write_uint32($this->fp);
		return $this->stream->read_bool();
	}

	public function lock($operation)
	{
		//不支持加锁
		return false;
	}
	
	public function seek($offset, $whence)
    {
		switch ($whence) {
		case SEEK_SET: $mode = 0; break;
		case SEEK_CUR: $mode = 1; break;
		case SEEK_END: $mode = 2; break;
		default : throw new Exception('seek whence error!');
		}
		//var_dump($offset, $mode);

		$this->stream->write_uint16(ByfsStream::CODE_FILE_SEEK);
		$this->stream->write_uint32($this->fp);
		$this->stream->write_int64($offset);
		$this->stream->write_uint8($mode);
		$ok = $this->stream->read_bool();
		if (!$ok) {
			$this->offset = -1;
			return -1;
		}
		$this->offset = $this->stream->read_int64();
		$this->eof = false;
		return 0;
    }

	public function tell()
	{
		if ($this->offset === null) {
			$this->seek(0, SEEK_CUR);
		}

		return $this->offset != -1 ? $this->offset : false;
    }

	public function stat()
	{
		$this->stream->write_uint16(ByfsStream::CODE_FILE_STAT);
		$this->stream->write_uint32($this->fp);
		$ok = $this->stream->read_bool();
		if (!$ok) {
			return false;
		}

		$is_dir = $this->stream->read_uint8();
		$size = $this->stream->read_int64();
		$modTime = $this->stream->read_int64();

		return ByfsFileSystem::_buildStat($is_dir, $size, $modTime);
	}

	public function truncate($new_size)
	{
		$this->stream->write_uint16(ByfsStream::CODE_FILE_TRUNCATE);
		$this->stream->write_uint32($this->fp);
		$this->stream->write_uint64($new_size);
		return $this->stream->read_bool();
	}

}

