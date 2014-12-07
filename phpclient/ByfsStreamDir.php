<?php

/**
 * 文件操作协议
 */
class ByfsStreamDir
{
	private $stream;
	private $hd;
	private $path;
	private $dirs;
	private $eof;
	private $cache_len = 100;

	public function __construct(ByfsStream $stream)
	{
		$this->stream = $stream;
	}

    public function open($path)
    {
		$this->stream->write_uint16(ByfsStream::CODE_DIR_OPEN);
		$this->stream->write_string($path);

		$ok = $this->stream->read_bool();
		if (!$ok) {
			return false;
		}

		$this->path = $path;
		$this->eof = false;
		$this->hd = $this->stream->read_uint32();

		return true;
    }

	public function __destruct()
	{
		$this->close();
	}

	public function close()
	{
		$this->dirs = array();

		if ($this->hd) {
			$this->stream->write_uint16(ByfsStream::CODE_DIR_CLOSE);
			$this->stream->write_uint32($this->hd);
			$this->hd = null;
			return $this->stream->read_bool();
		}
	}

	public function read()
	{
		if (!$this->eof && empty($this->dirs)) {
			$this->_read();
		}

		$dir = array_shift($this->dirs);
		return ($dir === null) ? false : $dir;
	}

    public function rewind()
    {
		$this->close();
		$this->open($this->path);

		return true;
    }

	public function _read()
    {
		$this->stream->write_uint16(ByfsStream::CODE_DIR_READ);
		$this->stream->write_uint32($this->hd);
		$this->stream->write_uint16($this->cache_len);
		$ok = $this->stream->read_bool();
		if (!$ok) {
			$this->eof = true;
			return
		}
		$this->dirs = $this->stream->read_array_string();

		if (count($this->dirs) < $this->cache_len) {
			$this->eof = true;
		}
    }

}

