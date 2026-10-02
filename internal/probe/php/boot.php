<?php
/*
 * boot — how long WordPress took to load (core + plugins + theme + init) before any page is built.
 * Args: none. The runner varies --skip-plugins / --skip-themes to isolate each component's cost.
 */
global $timestart;

echo "\n@@WPPERF@@" . wp_json_encode(
	array(
		'boot_ms'     => round( ( microtime( true ) - $timestart ) * 1000, 1 ),
		'peak_mem_mb' => round( memory_get_peak_usage() / 1048576, 1 ),
	)
) . "\n";
