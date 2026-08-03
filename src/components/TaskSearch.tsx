import { useEffect, useMemo, useRef, useState } from "react";
import { Search } from "lucide-react";
import type { Task } from "../domain/model";

const MAX_RESULTS = 7;

export function taskSearchResults(tasks: Task[], rawQuery: string): Task[] {
  const query = rawQuery.trim();
  if (!query) return [];

  if (query.startsWith("#")) {
    const match = query.match(/^#\s*(\d+)\s*$/);
    if (!match) return [];
    const publicId = Number(match[1]);
    return tasks.filter((task) => task.publicId === publicId).slice(0, 1);
  }

  const normalized = query.toLocaleLowerCase();
  return tasks
    .filter((task) => task.title.toLocaleLowerCase().includes(normalized))
    .sort((left, right) => {
      const leftStarts = left.title.toLocaleLowerCase().startsWith(normalized);
      const rightStarts = right.title.toLocaleLowerCase().startsWith(normalized);
      return Number(rightStarts) - Number(leftStarts) || left.publicId - right.publicId;
    })
    .slice(0, MAX_RESULTS);
}

export function TaskSearch({ tasks, onSelect }: { tasks: Task[]; onSelect: (task: Task, query: string) => void }) {
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const [open, setOpen] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const results = useMemo(() => taskSearchResults(tasks, query), [tasks, query]);
  const hasQuery = query.trim().length > 0;

  useEffect(() => setActiveIndex(0), [query]);

  const select = (task: Task) => {
    onSelect(task, query);
    setQuery(`#${task.publicId}`);
    setOpen(false);
    inputRef.current?.blur();
  };

  return (
    <div
      className="task-search nodrag nopan nowheel"
      onFocus={() => setOpen(true)}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false);
      }}
    >
      {open && hasQuery && (
        <div className="task-search-results" id="task-search-results" role="listbox" aria-label="Task 검색 결과">
          {results.length > 0 ? results.map((task, index) => (
            <button
              key={task.id}
              id={`task-search-result-${task.id}`}
              type="button"
              role="option"
              aria-selected={index === activeIndex}
              className={index === activeIndex ? "active" : ""}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => select(task)}
            >
              <span>#{task.publicId}</span>
              <strong>{task.title}</strong>
              <small>{task.status.replace("_", " ")}</small>
            </button>
          )) : <p>일치하는 Task가 없습니다.</p>}
        </div>
      )}
      <Search size={18} aria-hidden="true" />
      <input
        ref={inputRef}
        type="search"
        value={query}
        placeholder="#번호 또는 Task 제목 검색"
        aria-label="Task 검색"
        role="combobox"
        aria-expanded={open && hasQuery}
        aria-controls="task-search-results"
        aria-activedescendant={open && results[activeIndex] ? `task-search-result-${results[activeIndex]!.id}` : undefined}
        autoComplete="off"
        spellCheck={false}
        onChange={(event) => {
          setQuery(event.target.value);
          setOpen(true);
        }}
        onKeyDown={(event) => {
          if (event.key === "ArrowDown") {
            event.preventDefault();
            setOpen(true);
            setActiveIndex((current) => results.length ? (current + 1) % results.length : 0);
          } else if (event.key === "ArrowUp") {
            event.preventDefault();
            setOpen(true);
            setActiveIndex((current) => results.length ? (current - 1 + results.length) % results.length : 0);
          } else if (event.key === "Enter" && results[activeIndex]) {
            event.preventDefault();
            select(results[activeIndex]!);
          } else if (event.key === "Escape") {
            setQuery("");
            setOpen(false);
            inputRef.current?.blur();
          }
        }}
      />
    </div>
  );
}
