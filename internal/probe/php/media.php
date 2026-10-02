<?php
/*
 * media — walks the uploads folder: total size, file types, oversized originals,
 * and files that are not part of the media library.
 * Args: none. Read-only. Runs in the CLI, so there is no web timeout on large folders.
 */
global $wpdb;

$uploads = wp_upload_dir();
$base    = rtrim( $uploads['basedir'], '/' );

// Every file the media library knows about, relative to the uploads folder.
$library = array_flip( $wpdb->get_col( "SELECT meta_value FROM {$wpdb->postmeta} WHERE meta_key = '_wp_attached_file'" ) );

// Since WP 5.3 big images are scaled down; the untouched original is kept as "original_image".
foreach ( $wpdb->get_col( "SELECT meta_value FROM {$wpdb->postmeta} WHERE meta_key = '_wp_attachment_metadata'" ) as $raw ) {
	$meta = maybe_unserialize( $raw );
	if ( is_array( $meta ) && ! empty( $meta['original_image'] ) && ! empty( $meta['file'] ) ) {
		$dir = dirname( $meta['file'] );
		$library[ ( '.' === $dir ? '' : $dir . '/' ) . $meta['original_image'] ] = true;
	}
}

$images        = array( 'jpg', 'jpeg', 'png', 'gif', 'webp', 'avif' );
$total_bytes   = 0;
$total_files   = 0;
$by_ext        = array();
$big           = array();
$big_bytes     = 0;
$outside       = array();
$outside_files = 0;
$outside_bytes = 0;

if ( is_dir( $base ) ) {
	$it = new RecursiveIteratorIterator( new RecursiveDirectoryIterator( $base, FilesystemIterator::SKIP_DOTS ) );
	foreach ( $it as $file ) {
		if ( ! $file->isFile() ) {
			continue;
		}
		$size = $file->getSize();
		$rel  = ltrim( substr( $file->getPathname(), strlen( $base ) ), '/' );
		$ext  = strtolower( $file->getExtension() );

		$total_bytes += $size;
		++$total_files;
		if ( ! isset( $by_ext[ $ext ] ) ) {
			$by_ext[ $ext ] = array( 'name' => $ext, 'files' => 0, 'bytes' => 0 );
		}
		++$by_ext[ $ext ]['files'];
		$by_ext[ $ext ]['bytes'] += $size;

		if ( isset( $library[ $rel ] ) ) {
			if ( in_array( $ext, $images, true ) && $size > 1048576 ) {
				$big[]      = array( 'name' => $rel, 'bytes' => $size );
				$big_bytes += $size;
			}
			continue;
		}

		// Generated sizes (photo-300x200.jpg) belong to a library item.
		if ( preg_match( '/-\d+x\d+\.[a-z0-9]+$/i', $rel ) ) {
			continue;
		}
		// Folder guards most plugins drop in.
		if ( in_array( basename( $rel ), array( 'index.php', 'index.html', '.htaccess', 'web.config', 'robots.txt' ), true ) ) {
			continue;
		}

		$top = false !== strpos( $rel, '/' ) ? strstr( $rel, '/', true ) : '(uploads root)';
		if ( ! isset( $outside[ $top ] ) ) {
			$outside[ $top ] = array( 'name' => $top, 'files' => 0, 'bytes' => 0 );
		}
		++$outside[ $top ]['files'];
		$outside[ $top ]['bytes'] += $size;
		++$outside_files;
		$outside_bytes += $size;
	}
}

$by_bytes = function ( $a, $b ) {
	return $b['bytes'] <=> $a['bytes'];
};
usort( $big, $by_bytes );
$by_ext = array_values( $by_ext );
usort( $by_ext, $by_bytes );
$outside = array_values( $outside );
usort( $outside, $by_bytes );

$out = array(
	'uploads_dir'         => $base,
	'total_bytes'         => $total_bytes,
	'total_files'         => $total_files,
	'library_items'       => (int) $wpdb->get_var( "SELECT COUNT(*) FROM {$wpdb->posts} WHERE post_type = 'attachment'" ),
	'by_extension'        => array_slice( $by_ext, 0, 10 ),
	'oversized_count'     => count( $big ),
	'oversized_bytes'     => $big_bytes,
	'oversized_top'       => array_slice( $big, 0, 10 ),
	'outside_files'       => $outside_files,
	'outside_bytes'       => $outside_bytes,
	'outside_by_folder'   => array_slice( $outside, 0, 10 ),
);

echo "\n@@WPPERF@@" . wp_json_encode( $out ) . "\n";
