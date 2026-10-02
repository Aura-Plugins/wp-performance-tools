<?php
/*
 * discover — picks representative URLs to benchmark: the home page, the latest item of
 * every public post type, and a search.
 * Args: none. Read-only.
 */

$pages = array(
	array( 'label' => 'Home', 'path' => '/' ),
);

foreach ( get_post_types( array( 'public' => true ), 'objects' ) as $type ) {
	if ( 'attachment' === $type->name ) {
		continue;
	}
	$ids = get_posts(
		array(
			'post_type'        => $type->name,
			'post_status'      => 'publish',
			'posts_per_page'   => 1,
			'fields'           => 'ids',
			'orderby'          => 'date',
			'order'            => 'DESC',
			'suppress_filters' => true,
		)
	);
	if ( ! $ids ) {
		continue;
	}
	$path = wp_make_link_relative( get_permalink( $ids[0] ) );
	$pages[] = array(
		'label' => $type->labels->singular_name,
		'path'  => '' === $path ? '/' : $path,
	);
}

$pages[] = array( 'label' => 'Search', 'path' => '/?s=a' );

echo "\n@@WPPERF@@" . wp_json_encode( array( 'pages' => $pages ) ) . "\n";
