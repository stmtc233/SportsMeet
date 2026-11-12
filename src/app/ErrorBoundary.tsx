import { Component, type ErrorInfo, type ReactNode } from "react";
import { StButton, StEmpty } from "../design";

/**
 * Catches an error thrown while a page renders, so the window shows a way
 * back instead of going blank. `onReset` leaves the page that failed.
 */
export class ErrorBoundary extends Component<{ children: ReactNode; onReset: () => void }, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(error, info.componentStack);
  }

  render() {
    if (!this.state.failed) return this.props.children;
    const reset = () => {
      this.props.onReset();
      this.setState({ failed: false });
    };
    return (
      <div className="app__failed">
        <StEmpty action={<StButton variant="primary" onClick={reset}>返回全部赛事</StButton>}>页面出错，未保存的改动已丢失</StEmpty>
      </div>
    );
  }
}
