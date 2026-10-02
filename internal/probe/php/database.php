<?php
/*
 * database — table sizes, the heaviest post-meta keys, autoloaded options and clutter.
 * Args: none. Read-only.
 */
global $wpdb;

$tables = $wpdb->get_results(
	$wpdb->prepare(
		'SELECT table_name AS name, table_rows AS row_count, data_length AS data_bytes, index_length AS index_bytes
		 FROM information_schema.tables
		 WHERE table_schema = %s AND table_name LIKE %s
		 ORDER BY data_length + index_length DESC',
		DB_NAME,
		$wpdb->esc_like( $wpdb->prefix ) . '%'
	),
	ARRAY_A
);

// Post meta: total bytes and the keys that weigh the most.
$meta_total = (int) $wpdb->get_var( "SELECT COALESCE(SUM(LENGTH(meta_value)), 0) FROM {$wpdb->postmeta}" );
$meta_keys  = $wpdb->get_results(
	"SELECT meta_key AS name, COUNT(*) AS row_count, SUM(LENGTH(meta_value)) AS bytes
	 FROM {$wpdb->postmeta} GROUP BY meta_key ORDER BY bytes DESC LIMIT 10",
	ARRAY_A
);

// Autoloaded options are read on every single request.
$auto_values = function_exists( 'wp_autoload_values_to_autoload' ) ? wp_autoload_values_to_autoload() : array( 'yes' );
$auto_in     = "'" . implode( "','", array_map( 'esc_sql', $auto_values ) ) . "'";
$autoload    = $wpdb->get_row(
	"SELECT COUNT(*) AS option_count, COALESCE(SUM(LENGTH(option_value)), 0) AS bytes
	 FROM {$wpdb->options} WHERE autoload IN ($auto_in)",
	ARRAY_A
);
$autoload_top = $wpdb->get_results(
	"SELECT option_name AS name, LENGTH(option_value) AS bytes
	 FROM {$wpdb->options} WHERE autoload IN ($auto_in) ORDER BY bytes DESC LIMIT 10",
	ARRAY_A
);

// Clutter.
$now        = time();
$transients = (int) $wpdb->get_var(
	"SELECT COUNT(*) FROM {$wpdb->options}
	 WHERE option_name LIKE '\\_transient\\_%' OR option_name LIKE '\\_site\\_transient\\_%'"
);
$expired    = (int) $wpdb->get_var(
	"SELECT COUNT(*) FROM {$wpdb->options}
	 WHERE (option_name LIKE '\\_transient\\_timeout\\_%' OR option_name LIKE '\\_site\\_transient\\_timeout\\_%')
	 AND option_value < {$now}"
);
$orphan_meta = (int) $wpdb->get_var(
	"SELECT COUNT(*) FROM {$wpdb->postmeta} m LEFT JOIN {$wpdb->posts} p ON p.ID = m.post_id WHERE p.ID IS NULL"
);
$posts = $wpdb->get_results(
	"SELECT post_type AS type, post_status AS status, COUNT(*) AS count
	 FROM {$wpdb->posts} GROUP BY post_type, post_status ORDER BY count DESC",
	ARRAY_A
);

$out = array(
	'tables'           => $tables,
	'meta_total_bytes' => $meta_total,
	'meta_keys'        => $meta_keys,
	'autoload'         => $autoload,
	'autoload_top'     => $autoload_top,
	'transients'       => $transients,
	'expired_timeouts' => $expired,
	'orphan_meta'      => $orphan_meta,
	'posts'            => $posts,
);

echo "\n@@WPPERF@@" . wp_json_encode( $out ) . "\n";
