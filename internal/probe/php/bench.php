<?php
/*
 * bench — renders one front-end URL inside WordPress and reports what it cost:
 * time (bootstrap vs render), queries, repeated-query patterns, memory, and the post meta it loaded.
 *
 * Args: <path> [html]
 *   path  URL path to render, e.g. / or /news/my-post/ or /?s=term
 *   html  also return the rendered HTML (base64), used by the frontend scanner
 *
 * The runner adds --exec="define('SAVEQUERIES', true);" so every query is recorded.
 * WARNING: rendering runs the site's own code for that page, including anything it hooks on
 * page load. Run this against a copy of the site, not production.
 */
global $wpdb, $timestart;

$path      = isset( $args[0] ) ? $args[0] : '/';
$want_html = isset( $args[1] ) && 'html' === $args[1];
$t_boot    = microtime( true );

// Pretend to be a normal GET request for $path.
$_SERVER['REQUEST_URI']    = $path;
$_SERVER['REQUEST_METHOD'] = 'GET';
$_SERVER['HTTP_HOST']      = (string) parse_url( home_url(), PHP_URL_HOST );
$_SERVER['SERVER_NAME']    = $_SERVER['HTTP_HOST'];
if ( 0 === strpos( home_url(), 'https://' ) ) {
	$_SERVER['HTTPS'] = 'on'; // so asset URLs come out as https://, as a browser would see them
}
$qs = parse_url( $path, PHP_URL_QUERY );
if ( $qs ) {
	parse_str( $qs, $_GET );
}

// Measurement harness only: a redirect must never end the process mid-measurement.
$redirect = '';
add_filter( 'redirect_canonical', '__return_false' );
add_filter( 'password_protected_is_active', '__return_false' ); // "Password Protected" plugin login wall
add_filter(
	'wp_redirect',
	function ( $location ) use ( &$redirect ) {
		$redirect = (string) $location;
		return false;
	},
	PHP_INT_MAX
);

if ( ! defined( 'WP_USE_THEMES' ) ) {
	define( 'WP_USE_THEMES', true );
}
wp();
ob_start();
require ABSPATH . WPINC . '/template-loader.php';
$html  = ob_get_clean();
$t_end = microtime( true );

// What ended up in the post-meta cache. WordPress loads ALL meta of a post the first time any
// field of it is read, so big unused values here are paid for on every request.
$meta_posts   = 0;
$meta_bytes   = 0;
$meta_keys    = array();
$meta_tracked = false;
$oc           = isset( $GLOBALS['wp_object_cache'] ) ? $GLOBALS['wp_object_cache'] : null;
if ( $oc && 'WP_Object_Cache' === get_class( $oc ) && property_exists( $oc, 'cache' ) ) {
	$meta_tracked = true;
	$cache        = Closure::bind( function () { return $this->cache; }, $oc, 'WP_Object_Cache' )();
	foreach ( isset( $cache['post_meta'] ) ? $cache['post_meta'] : array() as $post_meta ) {
		++$meta_posts;
		foreach ( (array) $post_meta as $key => $values ) {
			foreach ( (array) $values as $value ) {
				$len                = strlen( (string) $value );
				$meta_bytes        += $len;
				$meta_keys[ $key ]  = ( isset( $meta_keys[ $key ] ) ? $meta_keys[ $key ] : 0 ) + $len;
			}
		}
	}
}
arsort( $meta_keys );
$top_meta = array();
foreach ( array_slice( $meta_keys, 0, 5, true ) as $key => $bytes ) {
	$top_meta[] = array( 'name' => (string) $key, 'bytes' => $bytes );
}

// Queries: totals, repeated patterns, and the classic N+1 shapes.
$queries   = $wpdb->queries ? $wpdb->queries : array();
$query_sec = 0.0;
$patterns  = array();
$families  = array();
$family_caller = array();
$option_lookups = 0;
$post_loads     = 0;
$meta_loads     = 0;

$opt_re  = '/FROM ' . preg_quote( $wpdb->options, '/' ) . " WHERE option_name = '([^']+)'/";
$post_re = '/^SELECT \* FROM ' . preg_quote( $wpdb->posts, '/' ) . ' WHERE ID = \d+ LIMIT 1$/';
$meta_re = '/^SELECT post_id, meta_key, meta_value FROM ' . preg_quote( $wpdb->postmeta, '/' ) . ' WHERE post_id IN \(\d+\)/';

foreach ( $queries as $q ) {
	$sql        = trim( preg_replace( '/\s+/', ' ', $q[0] ) );
	$query_sec += $q[1];

	$norm = preg_replace( array( "/'[^']*'/", '/\b\d+\b/', '/\((\s*\?\s*,?)+\)/' ), array( '?', '?', '(?)' ), $sql );
	$norm = substr( $norm, 0, 160 );
	$patterns[ $norm ] = ( isset( $patterns[ $norm ] ) ? $patterns[ $norm ] : 0 ) + 1;

	if ( preg_match( $opt_re, $sql, $m ) ) {
		// An option read one at a time = it is not autoloaded. Group them into families
		// (first two segments of the name) so 300 ACF repeater rows show up as one culprit.
		++$option_lookups;
		$parts  = array_values( array_filter( explode( '_', preg_replace( '/\d+/', 'N', $m[1] ) ), 'strlen' ) );
		$family = implode( '_', array_slice( $parts, 0, 2 ) );
		$families[ $family ] = ( isset( $families[ $family ] ) ? $families[ $family ] : 0 ) + 1;
		if ( ! isset( $family_caller[ $family ] ) && ! empty( $q[2] ) ) {
			$frames                   = array_map( 'trim', explode( ',', $q[2] ) );
			$family_caller[ $family ] = implode( ' → ', array_slice( $frames, -4 ) );
		}
	}
	if ( preg_match( $post_re, $sql ) ) {
		++$post_loads;
	}
	if ( preg_match( $meta_re, $sql ) ) {
		++$meta_loads;
	}
}

arsort( $patterns );
arsort( $families );
$top_patterns = array();
foreach ( array_slice( $patterns, 0, 6, true ) as $sql => $count ) {
	$top_patterns[] = array( 'name' => $sql, 'count' => $count );
}
$top_families = array();
foreach ( array_slice( $families, 0, 6, true ) as $family => $count ) {
	$top_families[] = array(
		'name'   => $family,
		'count'  => $count,
		'caller' => isset( $family_caller[ $family ] ) ? $family_caller[ $family ] : '',
	);
}

usort(
	$queries,
	function ( $a, $b ) {
		return $b[1] <=> $a[1];
	}
);
$slowest = array();
foreach ( array_slice( $queries, 0, 5 ) as $q ) {
	$slowest[] = array(
		'ms'  => round( $q[1] * 1000, 2 ),
		'sql' => substr( trim( preg_replace( '/\s+/', ' ', $q[0] ) ), 0, 300 ),
	);
}

$out = array(
	'path'            => $path,
	'status'          => is_404() ? 404 : 200,
	'template'        => isset( $template ) ? basename( (string) $template ) : '',
	'redirect'        => $redirect,
	'total_ms'        => round( ( $t_end - $timestart ) * 1000, 1 ),
	'bootstrap_ms'    => round( ( $t_boot - $timestart ) * 1000, 1 ),
	'render_ms'       => round( ( $t_end - $t_boot ) * 1000, 1 ),
	'queries'         => count( $queries ),
	'query_ms'        => round( $query_sec * 1000, 1 ),
	'peak_mem_mb'     => round( memory_get_peak_usage() / 1048576, 1 ),
	'html_kb'         => round( strlen( $html ) / 1024, 1 ),
	'meta_tracked'    => $meta_tracked,
	'meta_posts'      => $meta_posts,
	'meta_kb'         => round( $meta_bytes / 1024, 1 ),
	'option_lookups'  => $option_lookups,
	'post_loads'      => $post_loads,
	'meta_loads'      => $meta_loads,
	'top_meta_keys'   => $top_meta,
	'option_families' => $top_families,
	'query_patterns'  => $top_patterns,
	'slowest'         => $slowest,
	'html'            => $want_html ? base64_encode( $html ) : '',
);

echo "\n@@WPPERF@@" . wp_json_encode( $out ) . "\n";
