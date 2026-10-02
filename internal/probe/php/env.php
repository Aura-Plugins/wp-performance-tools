<?php
/*
 * env — site inventory: versions, URLs, active plugins and theme, cache setup.
 * Args: none.
 */
global $wpdb, $wp_version;

$plugins = array();
foreach ( (array) get_option( 'active_plugins', array() ) as $file ) {
	// WP-CLI's --skip-plugins takes the plugin slug: its folder, or the file name for single-file plugins.
	$plugins[] = '.' === dirname( $file ) ? basename( $file, '.php' ) : dirname( $file );
}

$theme = wp_get_theme();

$out = array(
	'wp_version'     => $wp_version,
	'php_version'    => PHP_VERSION,
	'db_version'     => $wpdb->db_server_info(),
	'site_url'       => get_option( 'siteurl' ),
	'home_url'       => home_url( '/' ),
	'table_prefix'   => $wpdb->prefix,
	'multisite'      => is_multisite(),
	'active_plugins' => $plugins,
	'theme'          => $theme->get_stylesheet(),
	'theme_version'  => (string) $theme->get( 'Version' ),
	'object_cache'   => (bool) wp_using_ext_object_cache(),
	'opcache'        => function_exists( 'opcache_get_status' ) && (bool) ini_get( 'opcache.enable_cli' ),
	'permalinks'     => (string) get_option( 'permalink_structure' ),
);

echo "\n@@WPPERF@@" . wp_json_encode( $out ) . "\n";
