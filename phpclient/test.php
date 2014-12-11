<?php
echo "<pre>\n";

require "./ByfsFileSystem.php";
require "./ByfsStream.php";
require "./ByfsStreamFile.php";
require "./ByfsStreamWrapper.php";
require "./FileStreamWrapper.php";


ByfsFileSystem::init('127.0.0.1', '8080', 300, '');
ByfsStreamWrapper::register();

FileStreamWrapper::add_ghost_dir(__DIR__."/123", "byfs://123");
FileStreamWrapper::register();

$bo = is_file("./123/123.txt");
var_dump($bo);

$fp = fopen("./123/123.txt", "w");

if (!$fp) {
	die("not fp");
}

fwrite($fp, "234.txt");

ftruncate($fp, 0);

fseek($fp, 0, SEEK_SET);
fwrite($fp, "1234.txt2");

fclose($fp);

