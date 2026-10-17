(() => {
  const body = document.body;
  const search = document.querySelector("#guide-search");
  const darkButton = document.querySelector("#toggle-dark");
  const printButton = document.querySelector("#print-guide");
  const sections = [...document.querySelectorAll(".section")];
  const links = [...document.querySelectorAll(".sidebar a")];

  const setDarkMode = (enabled) => {
    body.classList.toggle("is-dark", enabled);
    localStorage.setItem("photobackup-guide-dark", enabled ? "1" : "0");
    if (darkButton) {
      darkButton.textContent = enabled ? "โหมดสว่าง" : "โหมดมืด";
    }
  };

  const savedTheme = localStorage.getItem("photobackup-guide-dark") === "1";
  setDarkMode(savedTheme);

  darkButton?.addEventListener("click", () => {
    setDarkMode(!body.classList.contains("is-dark"));
  });

  printButton?.addEventListener("click", () => {
    window.print();
  });

  search?.addEventListener("input", (event) => {
    const value = event.target.value.trim().toLowerCase();
    sections.forEach((section) => {
      const found = value === "" || section.textContent.toLowerCase().includes(value);
      section.classList.toggle("hidden-by-search", !found);
    });
  });

  const updateActiveLink = () => {
    const current = sections
      .filter((section) => section.getBoundingClientRect().top < 180)
      .pop();
    links.forEach((link) => {
      const active = current && link.getAttribute("href") === `#${current.id}`;
      link.classList.toggle("active", Boolean(active));
    });
  };

  document.addEventListener("scroll", updateActiveLink, { passive: true });
  updateActiveLink();

  document.querySelectorAll("pre").forEach((block) => {
    const wrapper = document.createElement("div");
    wrapper.className = "code-toolbar";
    block.parentNode.insertBefore(wrapper, block);
    wrapper.appendChild(block);

    const button = document.createElement("button");
    button.className = "copy-code";
    button.type = "button";
    button.textContent = "คัดลอก";
    button.addEventListener("click", async () => {
      await navigator.clipboard.writeText(block.innerText);
      button.textContent = "คัดลอกแล้ว";
      window.setTimeout(() => {
        button.textContent = "คัดลอก";
      }, 1200);
    });
    wrapper.appendChild(button);
  });
})();
