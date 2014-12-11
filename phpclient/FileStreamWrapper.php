<?php

/**
 * file间谍包装器
 *
 * 将一些特殊的文件无缝的转换到byfs中
 */
final class FileStreamWrapper
{
	//需要无缝转移的目录(绝对路径)
	static public $ghost_dir = array();

	private $fp;
	private $dir;

	static public function register()
	{
		stream_wrapper_unregister('file');
		stream_wrapper_register('file', __CLASS__);
	}

	static public function unregister()
	{
		stream_wrapper_restore('file');
	}

	static public function add_ghost_dir($dir, $to)
	{
		$_dir = self::_realpath($dir);

		self::$ghost_dir[$_dir] = $to;
	}

	public function dir_closedir()
	{
		return closedir($this->dir);
	}

	public function dir_opendir($path)
	{
		$path = $this->_realpath($path);

		self::unregister();
		$this->dir = opendir($path);
		self::register();

		return $this->dir ? true : false;
	}

	public function dir_readdir()
	{
		return readdir($this->dir);
	}

	public function dir_rewinddir()
	{
		return rewinddir($this->dir);
	}

	public function mkdir($path, $mode, $options)
	{
		$path = $this->_realpath($path);

		self::unregister();
		$result = mkdir($path, $mode, $options);
		self::register();
		return $result;
	}

	public function rename($from, $to)
	{
		$from = $this->_realpath($path, $from_is_ghost);
		$to = $this->_realpath($to, $to_is_ghost);

		if ($from_is_ghost == $to_is_ghost) {
			self::unregister();
			$result = rename($from, $to);
			self::register();
			return $result;
		}

		self::unregister();
		if (is_file($from)) {
			$result = $this->_move_file($from, $to);
		} else {
			$result = $this->_move_dir($from, $to);
		}
		self::register();
		return $result;
	}

	public function rmdir($path, $options)
	{
		$path = $this->_realpath($path);

		self::unregister();
		//不如道如何触发的
		if ($options & STREAM_MKDIR_RECURSIVE) {
			$result = $this->_recursive_rmdir($path);
		} else {
			$result = rmdir($path);
		}
		self::register();
		return $result;
	}

	//miss for stream_select()
	//public function stream_cast ($cast_as)

	public function stream_close()
	{
		fclose($this->fp);
	}

	public function stream_eof()
    {
		return feof($this->fp);
    }

	public function stream_flush()
	{
		return fflush($this->fp);
	}

	public function stream_lock($operation)
	{
		return flock($this->fp, $operation);
	}

	public function stream_metadata($path, $option, $value)
	{
		$path = $this->_realpath($path);

		self::unregister();
		switch ($option) {
		case STREAM_META_TOUCH : $result = touch($path, $value[0], $value[1]); break;
		case STREAM_META_OWNER_NAME:
		case STREAM_META_OWNER: $result = chown($path, $value); break;
		case STREAM_META_GROUP_NAME:
		case STREAM_META_GROUP: $result = chgrp($path, $value); break;
		case STREAM_META_ACCESS: $result = chmod($path, $value); break;
		default: throw new Exception('stream_metadata option not defined');
		}
		self::register();
		return $result;
	}

    public function stream_open($path, $mode, $options, &$opened_path)
    {
		$use_include_path = $options & STREAM_USE_PATH;
		$quiet = !($options & STREAM_REPORT_ERRORS);

		//不支持include_path
		if ($use_include_path) {
			if (!$quiet) {
				trigger_error('file wrapper not support include path!', E_USER_ERROR);
			}
			return false;
		}

		$path = $this->_realpath($path);

		self::unregister();
		if ($quiet) {
			$this->fp = @fopen($path, $mode, false);
		} else {
			$this->fp = fopen($path, $mode, false);
		}
		self::unregister();
		return $this->fp ? true : false;
    }

    public function stream_read($count)
    {
		return fread($this->fp, $count);
    }

	public function stream_seek($offset, $whence)
    {
		return fseek($this->fp, $offset, $whence);
    }

	// miss for stream
	//public function stream_set_option($option, $arg1, $arg2)

	public function stream_stat()
	{
		return fstat($this->fp);
	}

    public function stream_tell()
    {
		return ftell($this->fp);
    }

	public function stream_truncate($new_size)
	{
		return ftruncate($this->fp, $new_size);
	}

    public function stream_write($data)
    {
		return fwrite($this->fp, $data);
    }

	public function unlink($path)
	{
		$path = $this->_realpath($path);

		self::unregister();
		$result = unlink($path);
		self::register();
		return $result;
	}

	public function url_stat($path, $flag)
	{
		$link = $flag & STREAM_URL_STAT_LINK;
		//file_exists等检测函数需要无报错
		$quiet = $flag & STREAM_URL_STAT_QUIET;

		$path = $this->_realpath($path);

		self::unregister();
		if ($link) {
			$result = $quiet ? @lstat($path) : lstat($path);
		} else {
			$result = $quiet ? @stat($path) : stat($path);
		}
		self::register();
		return $result;
	}

	private function _realpath($path, &$ghost=null)
	{
		$path = self::_get_path($path);

		foreach (self::$ghost_dir as $dir => $to) {
			if (strpos($path, $dir) === 0) {
				$path = substr($path, strlen($dir));

				$ghost = true;
				return $to . '/'. ltrim($path, '/');
			}
		}

		$ghost = false;
		return $path;
	}

	static private function _get_path($path)
	{
		$tmp = parse_url($path);
		if (!$tmp || empty($tmp['path'])) {
			throw new Exception("path:{$path} error");
		}
		$host = isset($tmp['host']) ? $tmp['host'] : null;

		$path = $host . $tmp['path'];

		$path = str_replace("\\", '/', $path);

		if ($path[0] == '/') {
			return self::_fix_path($path);
		}

		return self::_fix_path(getcwd() .'/'.$path);
	}

	static private function _fix_path($path)
	{
		$tmp = explode('/', $path);

		$new = array();

		foreach ($tmp as $part) {
			switch ($part) {
			case '':
			case '.':
				break;
			case '..':
				array_pop($new);
			default:
				$new[] = $part;
			}
		}

		if ($path[0] == '/') {
			return '/'.implode('/', $new);
		}

		return implode('/', $new);
	}

	/**
	 * 递归删除文件
	 */
	private function _recursive_rmdir($path)
	{
		$hd = opendir($path);
		if (!$hd) { return false; }

		while (false !== ($dir = readdir($hd))) {
			if ($dir != '.' && $dir != '..') {
				$file = $path . '/' . $dir;
				if (is_dir($file)) {
					$ok = $this->_recursive_rmdir($file);
					if (!$ok) { return false; }
				} else {
					$ok = unlink($file);
					if (!$ok) { return false; }
				}
			}
		}

		return true;
	}

	private function _move_file($form, $to)
	{
		$ok = copy($from, $to);
		if (!$ok) { return false; }

		return unlink($from);
	}

	/**
	 * 递归移动文件
	 */
	private function _move_dir($from, $to)
	{
		$hd = opendir($from);
		if (!$hd) { return false; }

		if (!is_dir($to)) {
			$mode = fileperms($from);
			$ok = mkdir($to, $mode, true);
			if (!$ok) { return false; }
		}

		while (false !== ($dir = readdir($hd))) {
			if ($dir != '.' && $dir != '..') {
				$file = $path . '/' . $dir;
				$to_file = $to . '/' . $dir;

				if (is_dir($file)) {
					$ok = $this->_move_dir($file, $to_file);
					if (!$ok) { return false; }
				} else {
					$ok = copy($file, $to_file);
					if (!$ok) { return false; }

					$ok = unlink($file);
					if (!$ok) { return false; }
				}
			}
		}

		return true;
	}
}

