// Lightbox functionality
let lbGroup = [], lbIdx = 0;

function openLightbox(img) {
  const group = img.dataset.group;
  lbGroup = Array.from(document.querySelectorAll('img[data-group="' + group + '"]'));
  lbIdx = parseInt(img.dataset.idx) || 0;
  showLightboxImage();
  document.getElementById('lightbox').classList.add('open');
  document.body.style.overflow = 'hidden';
}

function closeLightbox(e) {
  if (e.target.id === 'lightbox' || e.target.classList.contains('lightbox-close')) {
    document.getElementById('lightbox').classList.remove('open');
    document.body.style.overflow = '';
  }
}

function navLightbox(dir, e) {
  e.stopPropagation();
  lbIdx = Math.max(0, Math.min(lbGroup.length - 1, lbIdx + dir));
  showLightboxImage();
}

function showLightboxImage() {
  const img = lbGroup[lbIdx];
  document.getElementById('lightbox-img').src = img.src;
  document.getElementById('lightbox-img').alt = img.alt;
  document.getElementById('lightbox-idx').textContent = lbIdx + 1;
  document.getElementById('lightbox-total').textContent = lbGroup.length;
  document.querySelector('.lightbox-nav.prev').disabled = lbIdx === 0;
  document.querySelector('.lightbox-nav.next').disabled = lbIdx === lbGroup.length - 1;
}

document.addEventListener('keydown', function(e) {
  if (!document.getElementById('lightbox').classList.contains('open')) return;
  if (e.key === 'Escape') { document.getElementById('lightbox').classList.remove('open'); document.body.style.overflow = ''; }
  if (e.key === 'ArrowLeft') navLightbox(-1, e);
  if (e.key === 'ArrowRight') navLightbox(1, e);
});

// Sidebar scroll tracking
(function() {
  const sections = document.querySelectorAll('.section[id]');
  const navLinks = document.querySelectorAll('.sidebar a');
  if (!sections.length || !navLinks.length) return;

  function updateActiveSection() {
    let current = '';
    sections.forEach(function(section) {
      const rect = section.getBoundingClientRect();
      if (rect.top <= 150) current = section.id;
    });
    navLinks.forEach(function(link) {
      link.classList.toggle('active', link.getAttribute('href') === '#' + current);
    });
  }

  window.addEventListener('scroll', updateActiveSection);
  updateActiveSection();
})();
