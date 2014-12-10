<?php

/**
 * byfs协议包装器
 */
final class ByfsStreamWrapper
{
	private $fp;
	private $dir;

	static public function register()
	{
		stream_wrapper_register('byfs', __CLASS__);
	}

	static public function unregister()
	{
		stream_wrapper_unregister('byfs');
	}

	public function dir_closedir()
	{
		$this->dir = null;
	}

	public function dir_opendir($path)
	{
		$path = substr($path, strlen("byfs://"));

		$this->dir = ByfsFileSystem::opendir($path);

		return $this->dir ? true : false;
	}

	public function dir_readdir()
	{
		return $this->dir->read();
	}

	public function dir_rewinddir()
	{
		return $this->dir->rewind();
	}

	public function mkdir($path, $mode, $options)
	{
		$path = substr($path, strlen("byfs://"));

		$ok = ByfsFileSystem::mkdir($path, $mode, $options);

		if (!$ok) {
			trigger_error("mkdir error! path:{$path}", E_USER_ERROR);
		}

		return $ok;
	}

	public function rename($from, $to)
	{
		$from = substr($from, strlen("byfs://"));
		$to = substr($to, strlen("byfs://"));

		$ok = ByfsFileSystem::rename($from, $to);

		if (!$ok) {
			trigger_error("rename error! path:{$from} to:{$to}", E_USER_ERROR);
		}

		return $ok;
	}

	public function rmdir($path, $options)
	{
		$path = substr($path, strlen("byfs://"));

		//不如道如何触发的
		$recursive = $options & STREAM_MKDIR_RECURSIVE;

		$ok = ByfsFileSystem::rmdir($path, $recursive);

		if (!$ok) {
			trigger_error("rmdir error! path:{$path}", E_USER_ERROR);
		}

		return $ok;
	}

	//miss for stream_select()
	//public function stream_cast ($cast_as)

	public function stream_close()
	{
		$this->fp = null;
	}

	public function stream_eof()
    {
		return $this->fp->eof();
    }

	public function stream_flush()
	{
		return $this->fp->flush();
	}

	public function stream_lock($operation)
	{
		return $this->fp->lock($operation);
	}

	public function stream_metadata($path, $option, $value)
	{
		$path = substr($path, strlen("byfs://"));
		return false;
	}

    public function stream_open($path, $mode, $options, &$opened_path)
    {
		$path = substr($path, strlen("byfs://"));

		//miss
		//$use_include_path = $options & STREAM_USE_PATH;
		$quiet = !($options & STREAM_REPORT_ERRORS);

		$this->fp = ByfsFileSystem::fopen($path, $mode);

		if (!$this->fp && !$quiet) {
			trigger_error("open error! file:{$path}", E_USER_ERROR);
		}

		return $this->fp ? true : false;
    }

    public function stream_read($count)
    {
		return $this->fp->read($count);
    }

	public function stream_seek($offset, $whence)
    {
		return $this->fp->seek($offset, $whence);
    }

	// miss for stream
	//public function stream_set_option($option, $arg1, $arg2)

	public function stream_stat()
	{
		return $this->fp->stat();
	}

    public function stream_tell()
    {
		return $this->fp->tell();
    }

	public function stream_truncate($new_size)
	{
		return $this->fp->truncate($new_size);
	}

    public function stream_write($data)
    {
		return $this->fp->write($data);
    }

	public function unlink($path)
	{
		$path = substr($path, strlen("byfs://"));

		$ok = ByfsFileSystem::unlink($path);

		if (!$ok) {
			trigger_error("unlink error! path:{$path}", E_USER_ERROR);
		}

		return $ok;
	}

	public function url_stat($path, $flag)
	{
		$path = substr($path, strlen("byfs://"));

		$link = $flag & STREAM_URL_STAT_LINK;
		//file_exists等检测函数需要无报错
		$quiet = $flag & STREAM_URL_STAT_QUIET;

		if ($link) {
			$ok = ByfsFileSystem::lstat($path);
		} else {
			$ok = ByfsFileSystem::stat($path);
		}

		if (!$ok && !$quiet) {
			trigger_error("unlink error! path:{$path}", E_USER_ERROR);
		}

		return $ok;
	}


}

