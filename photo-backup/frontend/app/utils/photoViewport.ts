export function capturePhotoViewport(root: HTMLElement | null, movingIDs: Set<number>) {
  if (!root || window.scrollY < 80) return () => {};
  const cards = [...root.querySelectorAll<HTMLElement>("[data-photo-id]")];
  const firstVisible = cards.findIndex((card) => card.getBoundingClientRect().bottom > 0);
  if (firstVisible < 0 || cards[firstVisible]!.getBoundingClientRect().top >= window.innerHeight) {
    return () => {};
  }
  const anchor = cards.slice(firstVisible).find((card) => !movingIDs.has(Number(card.dataset.photoId)));
  if (!anchor) return () => {};
  const top = anchor.getBoundingClientRect().top;
  return () => {
    if (anchor.isConnected) window.scrollBy(0, anchor.getBoundingClientRect().top - top);
  };
}
