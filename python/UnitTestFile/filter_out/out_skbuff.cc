/*
 *	Routines having to do with the 'struct sk_buff' memory handlers.
 *
 *	Authors:	Alan Cox <alan@lxorguk.ukuu.org.uk>
 *			Florian La Roche <rzsfl@rz.uni-sb.de>
 *
 *	Fixes:
 *		Alan Cox	:	Fixed the worst of the load
 *					balancer bugs.
 *		Dave Platt	:	Interrupt stacking fix.
 *	Richard Kooijman	:	Timestamp fixes.
 *		Alan Cox	:	Changed buffer format.
 *		Alan Cox	:	destructor hook for AF_UNIX etc.
 *		Linus Torvalds	:	Better skb_clone.
 *		Alan Cox	:	Added skb_copy.
 *		Alan Cox	:	Added all the changed routines Linus
 *					only put in the headers
 *		Ray VanTassle	:	Fixed --skb->lock in free
 *		Alan Cox	:	skb_copy copy arp field
 *		Andi Kleen	:	slabified it.
 *		Robert Olsson	:	Removed skb_head_pool
 *
 *	NOTE:
 *		The __skb_ routines should be called with interrupts
 *	disabled, or you better be *real* sure that the operation is atomic
 *	with respect to whatever list is being frobbed (e.g. via lock_sock()
 *	or via disabling bottom half handlers, etc).
 *
 *	This program is free software; you can redistribute it and/or
 *	modify it under the terms of the GNU General Public License
 *	as published by the Free Software Foundation; either version
 *	2 of the License, or (at your option) any later version.
 */

/*
 *	The functions in this file will not compile correctly with gcc 2.4.x
 */

#define pr_fmt(fmt) KBUILD_MODNAME ": " fmt

#include <linux/module.h>
#include <linux/types.h>
#include <linux/kernel.h>
#include <linux/kmemcheck.h>
#include <linux/mm.h>
#include <linux/interrupt.h>
#include <linux/in.h>
#include <linux/inet.h>
#include <linux/slab.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <linux/netdevice.h>
#ifdef CONFIG_NET_CLS_ACT
#include <net/pkt_sched.h>
#endif
#include <linux/string.h>
#include <linux/skbuff.h>
#include <linux/splice.h>
#include <linux/cache.h>
#include <linux/rtnetlink.h>
#include <linux/init.h>
#include <linux/scatterlist.h>
#include <linux/errqueue.h>
#include <linux/prefetch.h>
#include <linux/if_vlan.h>

#include <net/protocol.h>
#include <net/dst.h>
#include <net/sock.h>
#include <net/checksum.h>
#include <net/ip6_checksum.h>
#include <net/xfrm.h>

#include <asm/uaccess.h>
#include <trace/events/skb.h>
#include <linux/highmem.h>

struct kmem_cache *skbuff_head_cache __read_mostly;
static struct kmem_cache *skbuff_fclone_cache __read_mostly;

/**
 *	skb_panic - private function for out-of-line support
 *	@skb:	buffer
 *	@sz:	size
 *	@addr:	address
 *	@msg:	skb_over_panic or skb_under_panic
 *
 *	Out-of-line support for skb_put() and skb_push().
 *	Called via the wrapper skb_over_panic() or skb_under_panic().
 *	Keep out of line to prevent kernel bloat.
 *	__builtin_return_address is not used because it is not always reliable.
 */
static void skb_panic(struct sk_buff *skb, unsigned int sz, void *addr,
		      const char msg[])
{}

static void skb_over_panic(struct sk_buff *skb, unsigned int sz, void *addr)
{}

static void skb_under_panic(struct sk_buff *skb, unsigned int sz, void *addr)
{}

/*
 * kmalloc_reserve is a wrapper around kmalloc_node_track_caller that tells
 * the caller if emergency pfmemalloc reserves are being used. If it is and
 * the socket is later found to be SOCK_MEMALLOC then PFMEMALLOC reserves
 * may be used. Otherwise, the packet data may be discarded until enough
 * memory is free
 */
#define kmalloc_reserve(size, gfp, node, pfmemalloc) \
	 __kmalloc_reserve(size, gfp, node, _RET_IP_, pfmemalloc)

static void *__kmalloc_reserve(size_t size, gfp_t flags, int node,
			       unsigned long ip, bool *pfmemalloc)
{}

/* 	Allocate a new skbuff. We do this ourselves so we can fill in a few
 *	'private' fields and also do memory statistics to find all the
 *	[BEEP] leaks.
 *
 */

struct sk_buff *__alloc_skb_head(gfp_t gfp_mask, int node)
{}

/**
 *	__alloc_skb	-	allocate a network buffer
 *	@size: size to allocate
 *	@gfp_mask: allocation mask
 *	@flags: If SKB_ALLOC_FCLONE is set, allocate from fclone cache
 *		instead of head cache and allocate a cloned (child) skb.
 *		If SKB_ALLOC_RX is set, __GFP_MEMALLOC will be used for
 *		allocations in case the data is required for writeback
 *	@node: numa node to allocate memory on
 *
 *	Allocate a new &sk_buff. The returned buffer has no headroom and a
 *	tail room of at least size bytes. The object has a reference count
 *	of one. The return is the buffer. On a failure the return is %NULL.
 *
 *	Buffers may only be allocated from interrupts using a @gfp_mask of
 *	%GFP_ATOMIC.
 */
struct sk_buff *__alloc_skb(unsigned int size, gfp_t gfp_mask,
			    int flags, int node)
{}
EXPORT_SYMBOL(__alloc_skb);

/**
 * build_skb - build a network buffer
 * @data: data buffer provided by caller
 * @frag_size: size of fragment, or 0 if head was kmalloced
 *
 * Allocate a new &sk_buff. Caller provides space holding head and
 * skb_shared_info. @data must have been allocated by kmalloc() only if
 * @frag_size is 0, otherwise data should come from the page allocator.
 * The return is the new skb buffer.
 * On a failure the return is %NULL, and @data is not freed.
 * Notes :
 *  Before IO, driver allocates only data buffer where NIC put incoming frame
 *  Driver should add room at head (NET_SKB_PAD) and
 *  MUST add room at tail (SKB_DATA_ALIGN(skb_shared_info))
 *  After IO, driver calls build_skb(), to allocate sk_buff and populate it
 *  before giving packet to stack.
 *  RX rings only contains data buffers, not full skbs.
 */
struct sk_buff *build_skb(void *data, unsigned int frag_size)
{}
EXPORT_SYMBOL(build_skb);

struct netdev_alloc_cache {};
static DEFINE_PER_CPU(struct netdev_alloc_cache, netdev_alloc_cache);

static void *__netdev_alloc_frag(unsigned int fragsz, gfp_t gfp_mask)
{}

/**
 * netdev_alloc_frag - allocate a page fragment
 * @fragsz: fragment size
 *
 * Allocates a frag from a page for receive buffer.
 * Uses GFP_ATOMIC allocations.
 */
void *netdev_alloc_frag(unsigned int fragsz)
{}
EXPORT_SYMBOL(netdev_alloc_frag);

/**
 *	__netdev_alloc_skb - allocate an skbuff for rx on a specific device
 *	@dev: network device to receive on
 *	@length: length to allocate
 *	@gfp_mask: get_free_pages mask, passed to alloc_skb
 *
 *	Allocate a new &sk_buff and assign it a usage count of one. The
 *	buffer has unspecified headroom built in. Users should allocate
 *	the headroom they think they need without accounting for the
 *	built in space. The built in space is used for optimisations.
 *
 *	%NULL is returned if there is no free memory.
 */
struct sk_buff *__netdev_alloc_skb(struct net_device *dev,
				   unsigned int length, gfp_t gfp_mask)
{}
EXPORT_SYMBOL(__netdev_alloc_skb);

void skb_add_rx_frag(struct sk_buff *skb, int i, struct page *page, int off,
		     int size, unsigned int truesize)
{}
EXPORT_SYMBOL(skb_add_rx_frag);

void skb_coalesce_rx_frag(struct sk_buff *skb, int i, int size,
			  unsigned int truesize)
{}
EXPORT_SYMBOL(skb_coalesce_rx_frag);

static void skb_drop_list(struct sk_buff **listp)
{}

static inline void skb_drop_fraglist(struct sk_buff *skb)
{}

static void skb_clone_fraglist(struct sk_buff *skb)
{}

static void skb_free_head(struct sk_buff *skb)
{}

static void skb_release_data(struct sk_buff *skb)
{}

/*
 *	Free an skbuff by memory without cleaning the state.
 */
static void kfree_skbmem(struct sk_buff *skb)
{}

static void skb_release_head_state(struct sk_buff *skb)
{}

/* Free everything but the sk_buff shell. */
static void skb_release_all(struct sk_buff *skb)
{}

/**
 *	__kfree_skb - private function
 *	@skb: buffer
 *
 *	Free an sk_buff. Release anything attached to the buffer.
 *	Clean the state. This is an internal helper function. Users should
 *	always call kfree_skb
 */

void __kfree_skb(struct sk_buff *skb)
{}
EXPORT_SYMBOL(__kfree_skb);

/**
 *	kfree_skb - free an sk_buff
 *	@skb: buffer to free
 *
 *	Drop a reference to the buffer and free it if the usage count has
 *	hit zero.
 */
void kfree_skb(struct sk_buff *skb)
{}
EXPORT_SYMBOL(kfree_skb);

void kfree_skb_list(struct sk_buff *segs)
{}
EXPORT_SYMBOL(kfree_skb_list);

/**
 *	skb_tx_error - report an sk_buff xmit error
 *	@skb: buffer that triggered an error
 *
 *	Report xmit error if a device callback is tracking this skb.
 *	skb must be freed afterwards.
 */
void skb_tx_error(struct sk_buff *skb)
{}
EXPORT_SYMBOL(skb_tx_error);

/**
 *	consume_skb - free an skbuff
 *	@skb: buffer to free
 *
 *	Drop a ref to the buffer and free it if the usage count has hit zero
 *	Functions identically to kfree_skb, but kfree_skb assumes that the frame
 *	is being dropped after a failure and notes that
 */
void consume_skb(struct sk_buff *skb)
{}
EXPORT_SYMBOL(consume_skb);

static void __copy_skb_header(struct sk_buff *new, const struct sk_buff *old)
{}

/*
 * You should not add any new code to this function.  Add it to
 * __copy_skb_header above instead.
 */
static struct sk_buff *__skb_clone(struct sk_buff *n, struct sk_buff *skb)
{}

/**
 *	skb_morph	-	morph one skb into another
 *	@dst: the skb to receive the contents
 *	@src: the skb to supply the contents
 *
 *	This is identical to skb_clone except that the target skb is
 *	supplied by the user.
 *
 *	The target skb is returned upon exit.
 */
struct sk_buff *skb_morph(struct sk_buff *dst, struct sk_buff *src)
{}
EXPORT_SYMBOL_GPL(skb_morph);

/**
 *	skb_copy_ubufs	-	copy userspace skb frags buffers to kernel
 *	@skb: the skb to modify
 *	@gfp_mask: allocation priority
 *
 *	This must be called on SKBTX_DEV_ZEROCOPY skb.
 *	It will copy all frags into kernel and drop the reference
 *	to userspace pages.
 *
 *	If this function is called from an interrupt gfp_mask() must be
 *	%GFP_ATOMIC.
 *
 *	Returns 0 on success or a negative error code on failure
 *	to allocate kernel memory to copy to.
 */
int skb_copy_ubufs(struct sk_buff *skb, gfp_t gfp_mask)
{}
EXPORT_SYMBOL_GPL(skb_copy_ubufs);

/**
 *	skb_clone	-	duplicate an sk_buff
 *	@skb: buffer to clone
 *	@gfp_mask: allocation priority
 *
 *	Duplicate an &sk_buff. The new one is not owned by a socket. Both
 *	copies share the same packet data but not structure. The new
 *	buffer has a reference count of 1. If the allocation fails the
 *	function returns %NULL otherwise the new buffer is returned.
 *
 *	If this function is called from an interrupt gfp_mask() must be
 *	%GFP_ATOMIC.
 */

struct sk_buff *skb_clone(struct sk_buff *skb, gfp_t gfp_mask)
{}
EXPORT_SYMBOL(skb_clone);

static void skb_headers_offset_update(struct sk_buff *skb, int off)
{}

static void copy_skb_header(struct sk_buff *new, const struct sk_buff *old)
{}

static inline int skb_alloc_rx_flag(const struct sk_buff *skb)
{}

/**
 *	skb_copy	-	create private copy of an sk_buff
 *	@skb: buffer to copy
 *	@gfp_mask: allocation priority
 *
 *	Make a copy of both an &sk_buff and its data. This is used when the
 *	caller wishes to modify the data and needs a private copy of the
 *	data to alter. Returns %NULL on failure or the pointer to the buffer
 *	on success. The returned buffer has a reference count of 1.
 *
 *	As by-product this function converts non-linear &sk_buff to linear
 *	one, so that &sk_buff becomes completely private and caller is allowed
 *	to modify all the data of returned buffer. This means that this
 *	function is not recommended for use in circumstances when only
 *	header is going to be modified. Use pskb_copy() instead.
 */

struct sk_buff *skb_copy(const struct sk_buff *skb, gfp_t gfp_mask)
{}
EXPORT_SYMBOL(skb_copy);

/**
 *	__pskb_copy_fclone	-  create copy of an sk_buff with private head.
 *	@skb: buffer to copy
 *	@headroom: headroom of new skb
 *	@gfp_mask: allocation priority
 *	@fclone: if true allocate the copy of the skb from the fclone
 *	cache instead of the head cache; it is recommended to set this
 *	to true for the cases where the copy will likely be cloned
 *
 *	Make a copy of both an &sk_buff and part of its data, located
 *	in header. Fragmented data remain shared. This is used when
 *	the caller wishes to modify only header of &sk_buff and needs
 *	private copy of the header to alter. Returns %NULL on failure
 *	or the pointer to the buffer on success.
 *	The returned buffer has a reference count of 1.
 */

struct sk_buff *__pskb_copy_fclone(struct sk_buff *skb, int headroom,
				   gfp_t gfp_mask, bool fclone)
{}
EXPORT_SYMBOL(__pskb_copy_fclone);

/**
 *	pskb_expand_head - reallocate header of &sk_buff
 *	@skb: buffer to reallocate
 *	@nhead: room to add at head
 *	@ntail: room to add at tail
 *	@gfp_mask: allocation priority
 *
 *	Expands (or creates identical copy, if @nhead and @ntail are zero)
 *	header of @skb. &sk_buff itself is not changed. &sk_buff MUST have
 *	reference count of 1. Returns zero in the case of success or error,
 *	if expansion failed. In the last case, &sk_buff is not changed.
 *
 *	All the pointers pointing into skb header may change and must be
 *	reloaded after call to this function.
 */

int pskb_expand_head(struct sk_buff *skb, int nhead, int ntail,
		     gfp_t gfp_mask)
{}
EXPORT_SYMBOL(pskb_expand_head);

/* Make private copy of skb with writable head and some headroom */

struct sk_buff *skb_realloc_headroom(struct sk_buff *skb, unsigned int headroom)
{}
EXPORT_SYMBOL(skb_realloc_headroom);

/**
 *	skb_copy_expand	-	copy and expand sk_buff
 *	@skb: buffer to copy
 *	@newheadroom: new free bytes at head
 *	@newtailroom: new free bytes at tail
 *	@gfp_mask: allocation priority
 *
 *	Make a copy of both an &sk_buff and its data and while doing so
 *	allocate additional space.
 *
 *	This is used when the caller wishes to modify the data and needs a
 *	private copy of the data to alter as well as more space for new fields.
 *	Returns %NULL on failure or the pointer to the buffer
 *	on success. The returned buffer has a reference count of 1.
 *
 *	You must pass %GFP_ATOMIC as the allocation priority if this function
 *	is called from an interrupt.
 */
struct sk_buff *skb_copy_expand(const struct sk_buff *skb,
				int newheadroom, int newtailroom,
				gfp_t gfp_mask)
{}
EXPORT_SYMBOL(skb_copy_expand);

/**
 *	skb_pad			-	zero pad the tail of an skb
 *	@skb: buffer to pad
 *	@pad: space to pad
 *
 *	Ensure that a buffer is followed by a padding area that is zero
 *	filled. Used by network drivers which may DMA or transfer data
 *	beyond the buffer end onto the wire.
 *
 *	May return error in out of memory cases. The skb is freed on error.
 */

int skb_pad(struct sk_buff *skb, int pad)
{}
EXPORT_SYMBOL(skb_pad);

/**
 *	pskb_put - add data to the tail of a potentially fragmented buffer
 *	@skb: start of the buffer to use
 *	@tail: tail fragment of the buffer to use
 *	@len: amount of data to add
 *
 *	This function extends the used data area of the potentially
 *	fragmented buffer. @tail must be the last fragment of @skb -- or
 *	@skb itself. If this would exceed the total buffer size the kernel
 *	will panic. A pointer to the first byte of the extra data is
 *	returned.
 */

unsigned char *pskb_put(struct sk_buff *skb, struct sk_buff *tail, int len)
{}
EXPORT_SYMBOL_GPL(pskb_put);

/**
 *	skb_put - add data to a buffer
 *	@skb: buffer to use
 *	@len: amount of data to add
 *
 *	This function extends the used data area of the buffer. If this would
 *	exceed the total buffer size the kernel will panic. A pointer to the
 *	first byte of the extra data is returned.
 */
unsigned char *skb_put(struct sk_buff *skb, unsigned int len)
{}
EXPORT_SYMBOL(skb_put);

/**
 *	skb_push - add data to the start of a buffer
 *	@skb: buffer to use
 *	@len: amount of data to add
 *
 *	This function extends the used data area of the buffer at the buffer
 *	start. If this would exceed the total buffer headroom the kernel will
 *	panic. A pointer to the first byte of the extra data is returned.
 */
unsigned char *skb_push(struct sk_buff *skb, unsigned int len)
{}
EXPORT_SYMBOL(skb_push);

/**
 *	skb_pull - remove data from the start of a buffer
 *	@skb: buffer to use
 *	@len: amount of data to remove
 *
 *	This function removes data from the start of a buffer, returning
 *	the memory to the headroom. A pointer to the next data in the buffer
 *	is returned. Once the data has been pulled future pushes will overwrite
 *	the old data.
 */
unsigned char *skb_pull(struct sk_buff *skb, unsigned int len)
{}
EXPORT_SYMBOL(skb_pull);

/**
 *	skb_trim - remove end from a buffer
 *	@skb: buffer to alter
 *	@len: new length
 *
 *	Cut the length of a buffer down by removing data from the tail. If
 *	the buffer is already under the length specified it is not modified.
 *	The skb must be linear.
 */
void skb_trim(struct sk_buff *skb, unsigned int len)
{}
EXPORT_SYMBOL(skb_trim);

/* Trims skb to length len. It can change skb pointers.
 */

int ___pskb_trim(struct sk_buff *skb, unsigned int len)
{}
EXPORT_SYMBOL(___pskb_trim);

/**
 *	__pskb_pull_tail - advance tail of skb header
 *	@skb: buffer to reallocate
 *	@delta: number of bytes to advance tail
 *
 *	The function makes a sense only on a fragmented &sk_buff,
 *	it expands header moving its tail forward and copying necessary
 *	data from fragmented part.
 *
 *	&sk_buff MUST have reference count of 1.
 *
 *	Returns %NULL (and &sk_buff does not change) if pull failed
 *	or value of new tail of skb in the case of success.
 *
 *	All the pointers pointing into skb header may change and must be
 *	reloaded after call to this function.
 */

/* Moves tail of skb head forward, copying data from fragmented part,
 * when it is necessary.
 * 1. It may fail due to malloc failure.
 * 2. It may change skb pointers.
 *
 * It is pretty complicated. Luckily, it is called only in exceptional cases.
 */
unsigned char *__pskb_pull_tail(struct sk_buff *skb, int delta)
{}
EXPORT_SYMBOL(__pskb_pull_tail);

/**
 *	skb_copy_bits - copy bits from skb to kernel buffer
 *	@skb: source skb
 *	@offset: offset in source
 *	@to: destination buffer
 *	@len: number of bytes to copy
 *
 *	Copy the specified number of bytes from the source skb to the
 *	destination buffer.
 *
 *	CAUTION ! :
 *		If its prototype is ever changed,
 *		check arch/{}/net/{}.S files,
 *		since it is called from BPF assembly code.
 */
int skb_copy_bits(const struct sk_buff *skb, int offset, void *to, int len)
{}
EXPORT_SYMBOL(skb_copy_bits);

/*
 * Callback from splice_to_pipe(), if we need to release some pages
 * at the end of the spd in case we error'ed out in filling the pipe.
 */
static void sock_spd_release(struct splice_pipe_desc *spd, unsigned int i)
{}

static struct page *linear_to_page(struct page *page, unsigned int *len,
				   unsigned int *offset,
				   struct sock *sk)
{}

static bool spd_can_coalesce(const struct splice_pipe_desc *spd,
			     struct page *page,
			     unsigned int offset)
{}

/*
 * Fill page/offset/length into spd, if it can hold more pages.
 */
static bool spd_fill_page(struct splice_pipe_desc *spd,
			  struct pipe_inode_info *pipe, struct page *page,
			  unsigned int *len, unsigned int offset,
			  bool linear,
			  struct sock *sk)
{}

static bool __splice_segment(struct page *page, unsigned int poff,
			     unsigned int plen, unsigned int *off,
			     unsigned int *len,
			     struct splice_pipe_desc *spd, bool linear,
			     struct sock *sk,
			     struct pipe_inode_info *pipe)
{}

/*
 * Map linear and fragment data from the skb to spd. It reports true if the
 * pipe is full or if we already spliced the requested length.
 */
static bool __skb_splice_bits(struct sk_buff *skb, struct pipe_inode_info *pipe,
			      unsigned int *offset, unsigned int *len,
			      struct splice_pipe_desc *spd, struct sock *sk)
{}

/*
 * Map data from the skb to a pipe. Should handle both the linear part,
 * the fragments, and the frag list. It does NOT handle frag lists within
 * the frag list, if such a thing exists. We'd probably need to recurse to
 * handle that cleanly.
 */
int skb_splice_bits(struct sk_buff *skb, unsigned int offset,
		    struct pipe_inode_info *pipe, unsigned int tlen,
		    unsigned int flags)
{}

/**
 *	skb_store_bits - store bits from kernel buffer to skb
 *	@skb: destination buffer
 *	@offset: offset in destination
 *	@from: source buffer
 *	@len: number of bytes to copy
 *
 *	Copy the specified number of bytes from the source buffer to the
 *	destination skb.  This function handles all the messy bits of
 *	traversing fragment lists and such.
 */

int skb_store_bits(struct sk_buff *skb, int offset, const void *from, int len)
{}
EXPORT_SYMBOL(skb_store_bits);

/* Checksum skb data. */
__wsum __skb_checksum(const struct sk_buff *skb, int offset, int len,
		      __wsum csum, const struct skb_checksum_ops *ops)
{}
EXPORT_SYMBOL(__skb_checksum);

__wsum skb_checksum(const struct sk_buff *skb, int offset,
		    int len, __wsum csum)
{}
EXPORT_SYMBOL(skb_checksum);

/* Both of above in one bottle. */

__wsum skb_copy_and_csum_bits(const struct sk_buff *skb, int offset,
				    u8 *to, int len, __wsum csum)
{}
EXPORT_SYMBOL(skb_copy_and_csum_bits);

 /**
 *	skb_zerocopy_headlen - Calculate headroom needed for skb_zerocopy()
 *	@from: source buffer
 *
 *	Calculates the amount of linear headroom needed in the 'to' skb passed
 *	into skb_zerocopy().
 */
unsigned int
skb_zerocopy_headlen(const struct sk_buff *from)
{}
EXPORT_SYMBOL_GPL(skb_zerocopy_headlen);

/**
 *	skb_zerocopy - Zero copy skb to skb
 *	@to: destination buffer
 *	@from: source buffer
 *	@len: number of bytes to copy from source buffer
 *	@hlen: size of linear headroom in destination buffer
 *
 *	Copies up to `len` bytes from `from` to `to` by creating references
 *	to the frags in the source buffer.
 *
 *	The `hlen` as calculated by skb_zerocopy_headlen() specifies the
 *	headroom in the `to` buffer.
 *
 *	Return value:
 *	0: everything is OK
 *	-ENOMEM: couldn't orphan frags of @from due to lack of memory
 *	-EFAULT: skb_copy_bits() found some problem with skb geometry
 */
int
skb_zerocopy(struct sk_buff *to, struct sk_buff *from, int len, int hlen)
{}
EXPORT_SYMBOL_GPL(skb_zerocopy);

void skb_copy_and_csum_dev(const struct sk_buff *skb, u8 *to)
{}
EXPORT_SYMBOL(skb_copy_and_csum_dev);

/**
 *	skb_dequeue - remove from the head of the queue
 *	@list: list to dequeue from
 *
 *	Remove the head of the list. The list lock is taken so the function
 *	may be used safely with other locking list functions. The head item is
 *	returned or %NULL if the list is empty.
 */

struct sk_buff *skb_dequeue(struct sk_buff_head *list)
{}
EXPORT_SYMBOL(skb_dequeue);

/**
 *	skb_dequeue_tail - remove from the tail of the queue
 *	@list: list to dequeue from
 *
 *	Remove the tail of the list. The list lock is taken so the function
 *	may be used safely with other locking list functions. The tail item is
 *	returned or %NULL if the list is empty.
 */
struct sk_buff *skb_dequeue_tail(struct sk_buff_head *list)
{}
EXPORT_SYMBOL(skb_dequeue_tail);

/**
 *	skb_queue_purge - empty a list
 *	@list: list to empty
 *
 *	Delete all buffers on an &sk_buff list. Each buffer is removed from
 *	the list and one reference dropped. This function takes the list
 *	lock and is atomic with respect to other list locking functions.
 */
void skb_queue_purge(struct sk_buff_head *list)
{}
EXPORT_SYMBOL(skb_queue_purge);

/**
 *	skb_queue_head - queue a buffer at the list head
 *	@list: list to use
 *	@newsk: buffer to queue
 *
 *	Queue a buffer at the start of the list. This function takes the
 *	list lock and can be used safely with other locking &sk_buff functions
 *	safely.
 *
 *	A buffer cannot be placed on two lists at the same time.
 */
void skb_queue_head(struct sk_buff_head *list, struct sk_buff *newsk)
{}
EXPORT_SYMBOL(skb_queue_head);

/**
 *	skb_queue_tail - queue a buffer at the list tail
 *	@list: list to use
 *	@newsk: buffer to queue
 *
 *	Queue a buffer at the tail of the list. This function takes the
 *	list lock and can be used safely with other locking &sk_buff functions
 *	safely.
 *
 *	A buffer cannot be placed on two lists at the same time.
 */
void skb_queue_tail(struct sk_buff_head *list, struct sk_buff *newsk)
{}
EXPORT_SYMBOL(skb_queue_tail);

/**
 *	skb_unlink	-	remove a buffer from a list
 *	@skb: buffer to remove
 *	@list: list to use
 *
 *	Remove a packet from a list. The list locks are taken and this
 *	function is atomic with respect to other list locked calls
 *
 *	You must know what list the SKB is on.
 */
void skb_unlink(struct sk_buff *skb, struct sk_buff_head *list)
{}
EXPORT_SYMBOL(skb_unlink);

/**
 *	skb_append	-	append a buffer
 *	@old: buffer to insert after
 *	@newsk: buffer to insert
 *	@list: list to use
 *
 *	Place a packet after a given packet in a list. The list locks are taken
 *	and this function is atomic with respect to other list locked calls.
 *	A buffer cannot be placed on two lists at the same time.
 */
void skb_append(struct sk_buff *old, struct sk_buff *newsk, struct sk_buff_head *list)
{}
EXPORT_SYMBOL(skb_append);

/**
 *	skb_insert	-	insert a buffer
 *	@old: buffer to insert before
 *	@newsk: buffer to insert
 *	@list: list to use
 *
 *	Place a packet before a given packet in a list. The list locks are
 * 	taken and this function is atomic with respect to other list locked
 *	calls.
 *
 *	A buffer cannot be placed on two lists at the same time.
 */
void skb_insert(struct sk_buff *old, struct sk_buff *newsk, struct sk_buff_head *list)
{}
EXPORT_SYMBOL(skb_insert);

static inline void skb_split_inside_header(struct sk_buff *skb,
					   struct sk_buff* skb1,
					   const u32 len, const int pos)
{}

static inline void skb_split_no_header(struct sk_buff *skb,
				       struct sk_buff* skb1,
				       const u32 len, int pos)
{}

/**
 * skb_split - Split fragmented skb to two parts at length len.
 * @skb: the buffer to split
 * @skb1: the buffer to receive the second part
 * @len: new length for skb
 */
void skb_split(struct sk_buff *skb, struct sk_buff *skb1, const u32 len)
{}
EXPORT_SYMBOL(skb_split);

/* Shifting from/to a cloned skb is a no-go.
 *
 * Caller cannot keep skb_shinfo related pointers past calling here!
 */
static int skb_prepare_for_shift(struct sk_buff *skb)
{}

/**
 * skb_shift - Shifts paged data partially from skb to another
 * @tgt: buffer into which tail data gets added
 * @skb: buffer from which the paged data comes from
 * @shiftlen: shift up to this many bytes
 *
 * Attempts to shift up to shiftlen worth of bytes, which may be less than
 * the length of the skb, from skb to tgt. Returns number bytes shifted.
 * It's up to caller to free skb if everything was shifted.
 *
 * If @tgt runs out of frags, the whole operation is aborted.
 *
 * Skb cannot include anything else but paged data while tgt is allowed
 * to have non-paged data as well.
 *
 * TODO: full sized shift could be optimized but that would need
 * specialized skb free'er to handle frags without up-to-date nr_frags.
 */
int skb_shift(struct sk_buff *tgt, struct sk_buff *skb, int shiftlen)
{}

/**
 * skb_prepare_seq_read - Prepare a sequential read of skb data
 * @skb: the buffer to read
 * @from: lower offset of data to be read
 * @to: upper offset of data to be read
 * @st: state variable
 *
 * Initializes the specified state variable. Must be called before
 * invoking skb_seq_read() for the first time.
 */
void skb_prepare_seq_read(struct sk_buff *skb, unsigned int from,
			  unsigned int to, struct skb_seq_state *st)
{}
EXPORT_SYMBOL(skb_prepare_seq_read);

/**
 * skb_seq_read - Sequentially read skb data
 * @consumed: number of bytes consumed by the caller so far
 * @data: destination pointer for data to be returned
 * @st: state variable
 *
 * Reads a block of skb data at @consumed relative to the
 * lower offset specified to skb_prepare_seq_read(). Assigns
 * the head of the data block to @data and returns the length
 * of the block or 0 if the end of the skb data or the upper
 * offset has been reached.
 *
 * The caller is not required to consume all of the data
 * returned, i.e. @consumed is typically set to the number
 * of bytes already consumed and the next call to
 * skb_seq_read() will return the remaining part of the block.
 *
 * Note 1: The size of each block of data returned can be arbitrary,
 *       this limitation is the cost for zerocopy sequential
 *       reads of potentially non linear data.
 *
 * Note 2: Fragment lists within fragments are not implemented
 *       at the moment, state->root_skb could be replaced with
 *       a stack for this purpose.
 */
unsigned int skb_seq_read(unsigned int consumed, const u8 **data,
			  struct skb_seq_state *st)
{}
EXPORT_SYMBOL(skb_seq_read);

/**
 * skb_abort_seq_read - Abort a sequential read of skb data
 * @st: state variable
 *
 * Must be called if skb_seq_read() was not called until it
 * returned 0.
 */
void skb_abort_seq_read(struct skb_seq_state *st)
{}
EXPORT_SYMBOL(skb_abort_seq_read);

#define TS_SKB_CB(state)	((struct skb_seq_state *) &((state)->cb))

static unsigned int skb_ts_get_next_block(unsigned int offset, const u8 **text,
					  struct ts_config *conf,
					  struct ts_state *state)
{}

static void skb_ts_finish(struct ts_config *conf, struct ts_state *state)
{}

/**
 * skb_find_text - Find a text pattern in skb data
 * @skb: the buffer to look in
 * @from: search offset
 * @to: search limit
 * @config: textsearch configuration
 * @state: uninitialized textsearch state variable
 *
 * Finds a pattern in the skb data according to the specified
 * textsearch configuration. Use textsearch_next() to retrieve
 * subsequent occurrences of the pattern. Returns the offset
 * to the first occurrence or UINT_MAX if no match was found.
 */
unsigned int skb_find_text(struct sk_buff *skb, unsigned int from,
			   unsigned int to, struct ts_config *config,
			   struct ts_state *state)
{}
EXPORT_SYMBOL(skb_find_text);

/**
 * skb_append_datato_frags - append the user data to a skb
 * @sk: sock  structure
 * @skb: skb structure to be appended with user data.
 * @getfrag: call back function to be used for getting the user data
 * @from: pointer to user message iov
 * @length: length of the iov message
 *
 * Description: This procedure append the user data in the fragment part
 * of the skb if any page alloc fails user this procedure returns  -ENOMEM
 */
int skb_append_datato_frags(struct sock *sk, struct sk_buff *skb,
			int (*getfrag)(void *from, char *to, int offset,
					int len, int odd, struct sk_buff *skb),
			void *from, int length)
{}
EXPORT_SYMBOL(skb_append_datato_frags);

/**
 *	skb_pull_rcsum - pull skb and update receive checksum
 *	@skb: buffer to update
 *	@len: length of data pulled
 *
 *	This function performs an skb_pull on the packet and updates
 *	the CHECKSUM_COMPLETE checksum.  It should be used on
 *	receive path processing instead of skb_pull unless you know
 *	that the checksum difference is zero (e.g., a valid IP header)
 *	or you are setting ip_summed to CHECKSUM_NONE.
 */
unsigned char *skb_pull_rcsum(struct sk_buff *skb, unsigned int len)
{}
EXPORT_SYMBOL_GPL(skb_pull_rcsum);

/**
 *	skb_segment - Perform protocol segmentation on skb.
 *	@head_skb: buffer to segment
 *	@features: features for the output path (see dev->features)
 *
 *	This function performs segmentation on the given skb.  It returns
 *	a pointer to the first in a list of new skbs for the segments.
 *	In case of error it returns ERR_PTR(err).
 */
struct sk_buff *skb_segment(struct sk_buff *head_skb,
			    netdev_features_t features)
{}
EXPORT_SYMBOL_GPL(skb_segment);

int skb_gro_receive(struct sk_buff **head, struct sk_buff *skb)
{}
EXPORT_SYMBOL_GPL(skb_gro_receive);

void __init skb_init(void)
{}

/**
 *	skb_to_sgvec - Fill a scatter-gather list from a socket buffer
 *	@skb: Socket buffer containing the buffers to be mapped
 *	@sg: The scatter-gather list to map into
 *	@offset: The offset into the buffer's contents to start mapping
 *	@len: Length of buffer space to be mapped
 *
 *	Fill the specified scatter-gather list with mappings/pointers into a
 *	region of the buffer space attached to a socket buffer.
 */
static int
__skb_to_sgvec(struct sk_buff *skb, struct scatterlist *sg, int offset, int len)
{}

/* As compared with skb_to_sgvec, skb_to_sgvec_nomark only map skb to given
 * sglist without mark the sg which contain last skb data as the end.
 * So the caller can mannipulate sg list as will when padding new data after
 * the first call without calling sg_unmark_end to expend sg list.
 *
 * Scenario to use skb_to_sgvec_nomark:
 * 1. sg_init_table
 * 2. skb_to_sgvec_nomark(payload1)
 * 3. skb_to_sgvec_nomark(payload2)
 *
 * This is equivalent to:
 * 1. sg_init_table
 * 2. skb_to_sgvec(payload1)
 * 3. sg_unmark_end
 * 4. skb_to_sgvec(payload2)
 *
 * When mapping mutilple payload conditionally, skb_to_sgvec_nomark
 * is more preferable.
 */
int skb_to_sgvec_nomark(struct sk_buff *skb, struct scatterlist *sg,
			int offset, int len)
{}
EXPORT_SYMBOL_GPL(skb_to_sgvec_nomark);

int skb_to_sgvec(struct sk_buff *skb, struct scatterlist *sg, int offset, int len)
{}
EXPORT_SYMBOL_GPL(skb_to_sgvec);

/**
 *	skb_cow_data - Check that a socket buffer's data buffers are writable
 *	@skb: The socket buffer to check.
 *	@tailbits: Amount of trailing space to be added
 *	@trailer: Returned pointer to the skb where the @tailbits space begins
 *
 *	Make sure that the data buffers attached to a socket buffer are
 *	writable. If they are not, private copies are made of the data buffers
 *	and the socket buffer is set to use these instead.
 *
 *	If @tailbits is given, make sure that there is space to write @tailbits
 *	bytes of data beyond current end of socket buffer.  @trailer will be
 *	set to point to the skb in which this space begins.
 *
 *	The number of scatterlist elements required to completely map the
 *	COW'd and extended socket buffer will be returned.
 */
int skb_cow_data(struct sk_buff *skb, int tailbits, struct sk_buff **trailer)
{}
EXPORT_SYMBOL_GPL(skb_cow_data);

static void sock_rmem_free(struct sk_buff *skb)
{}

/*
 * Note: We dont mem charge error packets (no sk_forward_alloc changes)
 */
int sock_queue_err_skb(struct sock *sk, struct sk_buff *skb)
{}
EXPORT_SYMBOL(sock_queue_err_skb);

void __skb_tstamp_tx(struct sk_buff *orig_skb,
		     struct skb_shared_hwtstamps *hwtstamps,
		     struct sock *sk, int tstype)
{}
EXPORT_SYMBOL_GPL(__skb_tstamp_tx);

void skb_tstamp_tx(struct sk_buff *orig_skb,
		   struct skb_shared_hwtstamps *hwtstamps)
{}
EXPORT_SYMBOL_GPL(skb_tstamp_tx);

void skb_complete_wifi_ack(struct sk_buff *skb, bool acked)
{}
EXPORT_SYMBOL_GPL(skb_complete_wifi_ack);


/**
 * skb_partial_csum_set - set up and verify partial csum values for packet
 * @skb: the skb to set
 * @start: the number of bytes after skb->data to start checksumming.
 * @off: the offset from start to place the checksum.
 *
 * For untrusted partially-checksummed packets, we need to make sure the values
 * for skb->csum_start and skb->csum_offset are valid so we don't oops.
 *
 * This function checks and sets those values and skb->ip_summed: if this
 * returns false you should drop the packet.
 */
bool skb_partial_csum_set(struct sk_buff *skb, u16 start, u16 off)
{}
EXPORT_SYMBOL_GPL(skb_partial_csum_set);

static int skb_maybe_pull_tail(struct sk_buff *skb, unsigned int len,
			       unsigned int max)
{}

#define MAX_TCP_HDR_LEN (15 * 4)

static __sum16 *skb_checksum_setup_ip(struct sk_buff *skb,
				      typeof(IPPROTO_IP) proto,
				      unsigned int off)
{}

/* This value should be large enough to cover a tagged ethernet header plus
 * maximally sized IP and TCP or UDP headers.
 */
#define MAX_IP_HDR_LEN 128

static int skb_checksum_setup_ipv4(struct sk_buff *skb, bool recalculate)
{}

/* This value should be large enough to cover a tagged ethernet header plus
 * an IPv6 header, all options, and a maximal TCP or UDP header.
 */
#define MAX_IPV6_HDR_LEN 256

#define OPT_HDR(type, skb, off) \
	(type *)(skb_network_header(skb) + (off))

static int skb_checksum_setup_ipv6(struct sk_buff *skb, bool recalculate)
{}

/**
 * skb_checksum_setup - set up partial checksum offset
 * @skb: the skb to set up
 * @recalculate: if true the pseudo-header checksum will be recalculated
 */
int skb_checksum_setup(struct sk_buff *skb, bool recalculate)
{}
EXPORT_SYMBOL(skb_checksum_setup);

void __skb_warn_lro_forwarding(const struct sk_buff *skb)
{}
EXPORT_SYMBOL(__skb_warn_lro_forwarding);

void kfree_skb_partial(struct sk_buff *skb, bool head_stolen)
{}
EXPORT_SYMBOL(kfree_skb_partial);

/**
 * skb_try_coalesce - try to merge skb to prior one
 * @to: prior buffer
 * @from: buffer to add
 * @fragstolen: pointer to boolean
 * @delta_truesize: how much more was allocated than was requested
 */
bool skb_try_coalesce(struct sk_buff *to, struct sk_buff *from,
		      bool *fragstolen, int *delta_truesize)
{}
EXPORT_SYMBOL(skb_try_coalesce);

/**
 * skb_scrub_packet - scrub an skb
 *
 * @skb: buffer to clean
 * @xnet: packet is crossing netns
 *
 * skb_scrub_packet can be used after encapsulating or decapsulting a packet
 * into/from a tunnel. Some information have to be cleared during these
 * operations.
 * skb_scrub_packet can also be used to clean a skb before injecting it in
 * another namespace (@xnet == true). We have to clear all information in the
 * skb that could impact namespace isolation.
 */
void skb_scrub_packet(struct sk_buff *skb, bool xnet)
{}
EXPORT_SYMBOL_GPL(skb_scrub_packet);

/**
 * skb_gso_transport_seglen - Return length of individual segments of a gso packet
 *
 * @skb: GSO skb
 *
 * skb_gso_transport_seglen is used to determine the real size of the
 * individual segments, including Layer4 headers (TCP/UDP).
 *
 * The MAC/L2 or network (IP, IPv6) headers are not accounted for.
 */
unsigned int skb_gso_transport_seglen(const struct sk_buff *skb)
{}
EXPORT_SYMBOL_GPL(skb_gso_transport_seglen);

static struct sk_buff *skb_reorder_vlan_header(struct sk_buff *skb)
{}

struct sk_buff *skb_vlan_untag(struct sk_buff *skb)
{}
EXPORT_SYMBOL(skb_vlan_untag);
