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
		$mode = trim(trim($mode), 'b');

		$flag = 0;
		switch ($mode) {
		case 'r' :
			$flag = ByfsStream::O_RDONLY;
		case 'r+' :
			$flag = ByfsStream::O_RDWR;
		case 'w' :
			$flag = ByfsStream::O_WRONLY | ByfsStream::O_TRUNC | ByfsStream::O_CREATE;
		case 'w+' :
			$flag = ByfsStream::O_RDWR | ByfsStream::O_TRUNC | ByfsStream::O_CREATE;
		case 'a' :
			$flag = ByfsStream::O_APPEND | ByfsStream::O_CREATE;
		case 'a+' :
			$flag = ByfsStream::O_RDWR | ByfsStream::O_CREATE;
		case 'x' :
			$flag = ByfsStream::O_WRONLY | ByfsStream::O_CREATE | ByfsStream::O_EXCL;
		case 'x+' :
			$flag = ByfsStream::O_RDWR | ByfsStream::O_CREATE | ByfsStream::O_EXCL;
		case 'c' :
			$flag = ByfsStream::O_WRONLY | ByfsStream::O_CREATE;
		case 'c+' :
			$flag = ByfsStream::O_RDWR | ByfsStream::O_CREATE;
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
				$this->seek(SEEK_END,0)
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
		$this->stream->write_uint32($count);
		$ok = $this->stream->read_bool();
		if (!$ok) {
			return false;
		}
		$this->eof = $this->stream->read_uint8();
		return $this->stream->read_string();
    }

    public function write($data)
	{
		$this->offset = null;
		$this->stream->write_uint16(ByfsStream::CODE_FILE_WRITE);
		$this->stream->write_uint32($this->fp);
		$this->stream->write_string($data);
		$ok = $this->stream->read_bool();
		if (!$ok) { return false; }
		return $this->stream->read_uint64();
    }

	public function eof()
    {
		return $this->eof != 0 ? true : false;
    }

	public function flush()
	{
		$this->stream->write_uint16(ByfsStream::CODE_FILE_FLUSH);
		$this->stream->write_uint32($this->fp);
		return $this->stream->read_bool();
	}

	public function lock($operation)
	{
		//解锁
		if ($operation & LOCK_UN) {
			$this->stream->write_uint16(ByfsStream::CODE_FILE_UNLOCK);
			$this->stream->write_uint32($this->fp);
			return $this->stream->read_bool();
		}

		$mode = 0;
		if ($operation & LOCK_SH) {
			$mode |= 1;
		}
		if ($operation & LOCK_EX) {
			$mode |= 2;
		}
		if ($operation & LOCK_NB) {
			$mode |= 4;
		}

		$this->stream->write_uint16(ByfsStream::CODE_FILE_LOCK);
		$this->stream->write_uint32($this->fp);
		$this->stream->write_uint8($mode);
		return $this->stream->read_bool();
	}
	
	public function seek($offset, $whence)
    {
		switch ($whence) {
		case SEEK_SET: $mode = 0; break;
		case SEEK_CUR: $mode = 1; break;
		case SEEK_END: $mode = 2; break;
		default : throw new Exception('seek whence error!');
		}

		$this->stream->write_uint16(ByfsStream::CODE_FILE_SEEK);
		$this->stream->write_uint32($this->fp);
		$this->stream->write_int64($offset);
		$this->stream->write_uint8($mode);
		$ok = $this->stream->read_bool();
		if (!$ok) {
			$this->offset = -1;
			return -1;
		}
		$this->offset = $this->stream->read_uint64();
		return 0;
    }

	public function tell()
	{
		if ($this->offset === null) {
			$this->seek(0, SEEK_CUR);
		}

		return $this->offset != -1 : $this->offset : false;
    }

	public function stat()
	{
		$this->stream->write_uint16(ByfsStream::CODE_FILE_STAT);
		$this->stream->write_uint32($this->fp);
		$ok = $this->stream->read_bool();
		if (!$ok) {
			return false;
		}
		$mode = $this->stream->read_uint32();
		$size = $this->stream->read_int64();
		$modTime = $this->stream->read_int64();

		$data = array(
			'dev' => 0,
			'ino' => 0
			'mode' => $mode,
			'nlink' => 0,
			'uid' => 0,
			'gid' => 0,
			'rdev' => 0,
			'size' => $size,
			'atime' => 0,
			'mtime' => $modTime,
			'ctime' => 0,
			'blksize' => 0,
			'blocks' => 0,
		);

		return $data;
	}

	public function truncate($new_size)
	{
		$this->stream->write_uint16(ByfsStream::CODE_FILE_TRUNCATE);
		$this->stream->write_uint32($this->fp);
		$this->stream->write_uint64($new_size);
		return $this->stream->read_bool();
	}

}

