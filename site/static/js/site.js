(function () {
  function flash(b, text) {
    var old = b.textContent;
    b.textContent = text;
    setTimeout(function () { b.textContent = old; }, 1500);
  }
  function copy(text, b) {
    navigator.clipboard.writeText(text).then(function () { flash(b, "Copied!"); });
  }

  document.querySelectorAll(".copy-btn[data-target]").forEach(function (b) {
    b.addEventListener("click", function () {
      copy(document.getElementById(b.dataset.target).textContent, b);
    });
  });

  document.querySelectorAll(".prose pre").forEach(function (pre) {
    var b = document.createElement("button");
    b.type = "button";
    b.className = "copy-btn pre-copy";
    b.textContent = "Copy";
    b.addEventListener("click", function () { copy(pre.innerText.replace(/\n$/, ""), b); });
    pre.appendChild(b);
  });

  var toc = document.querySelector(".toc");
  if (toc && "IntersectionObserver" in window) {
    var links = {};
    toc.querySelectorAll("a").forEach(function (a) { links[a.getAttribute("href").slice(1)] = a; });
    var obs = new IntersectionObserver(function (entries) {
      entries.forEach(function (e) {
        if (e.isIntersecting && links[e.target.id]) {
          toc.querySelectorAll("a.active").forEach(function (a) { a.classList.remove("active"); });
          links[e.target.id].classList.add("active");
        }
      });
    }, { rootMargin: "0px 0px -70% 0px" });
    document.querySelectorAll(".prose h2[id], .prose h3[id]").forEach(function (h) { obs.observe(h); });
  }
})();
