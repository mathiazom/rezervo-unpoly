(function () {
  const THRESHOLD = 80;

  function getSwipeLi(target) {
    return target.closest('[data-swipe-cancel]');
  }

  document.addEventListener('touchstart', function (e) {
    const li = getSwipeLi(e.target);
    if (!li) return;
    const t = e.touches[0];
    li._swipe = { startX: t.clientX, startY: t.clientY, dragging: false, cancelled: false };
  }, { passive: true });

  document.addEventListener('touchmove', function (e) {
    const li = getSwipeLi(e.target);
    if (!li || !li._swipe || li._swipe.cancelled) return;
    const t = e.touches[0];
    const dx = t.clientX - li._swipe.startX;
    const dy = t.clientY - li._swipe.startY;

    // Cancel swipe tracking if more vertical than horizontal
    if (!li._swipe.dragging && Math.abs(dy) > Math.abs(dx)) {
      li._swipe.cancelled = true;
      return;
    }
    // Only track leftward motion
    if (dx >= 0) return;

    li._swipe.dragging = true;
    e.preventDefault();
    const card = li.querySelector('a');
    card.style.transition = 'none';
    card.style.transform = 'translateX(' + Math.max(dx, -THRESHOLD * 1.5) + 'px)';
  }, { passive: false });

  document.addEventListener('touchend', function (e) {
    const li = getSwipeLi(e.target);
    if (!li || !li._swipe) return;
    const swipe = li._swipe;
    delete li._swipe;

    if (!swipe.dragging) return;

    const t = e.changedTouches[0];
    const dx = t.clientX - swipe.startX;
    const card = li.querySelector('a');

    // Snap back in all cases; open modal if past threshold
    card.style.transition = 'transform 0.2s ease';
    card.style.transform = 'translateX(0)';
    card.addEventListener('transitionend', function () {
      card.style.transition = '';
      card.style.transform = '';
    }, { once: true });

    if (dx < -THRESHOLD) {
      li._swiped = true; // suppress the subsequent click
      up.layer.open({
        url: '/cancel-modal/' + li.dataset.chain + '/' + li.dataset.classId
          + '?name=' + encodeURIComponent(li.dataset.activityName),
        mode: 'modal',
        history: false,
        size: 'medium',
      });
    }
  }, { passive: true });

  // Suppress click-to-detail when a swipe just occurred
  document.addEventListener('click', function (e) {
    const a = e.target.closest('a[data-detail-link]');
    if (!a) return;
    const li = a.closest('[data-swipe-cancel]');
    if (li && li._swiped) {
      e.preventDefault();
      e.stopImmediatePropagation();
      delete li._swiped;
    }
  }, { capture: true });
}());
